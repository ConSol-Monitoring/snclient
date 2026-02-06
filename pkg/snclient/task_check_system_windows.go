package snclient

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/shirou/gopsutil/v4/mem"
	"golang.org/x/sys/windows"
)

// diskPerformance is an equivalent representation of DISK_PERFORMANCE in the Windows API.
// https://docs.microsoft.com/fr-fr/windows/win32/api/winioctl/ns-winioctl-disk_performance
// Additional info: https://stackoverflow.com/questions/9258705/disk-performance-structs-readtime-and-writetime-members
type diskPerformance struct {
	BytesRead    int64
	BytesWritten int64
	// This has a different meaning on Windows/Non-Windows platforms.
	// According to Microsoft Docs: The time it takes to complete a read.
	// In our testing, it behaves like an always increasing counter, not related to the current load of the disk.
	ReadTime int64
	// This has a different meaning on Windows/Non-Windows platforms.
	// According to Microsoft Docs: The time it takes to complete a write.
	// In our testing, it behaves like an always increasing counter, not related to the current load of the disk.
	WriteTime int64
	// IdleTime: Total time the disk has been idle since boot (in 100-nanosecond increments).
	IdleTime   int64
	ReadCount  uint32
	WriteCount uint32
	QueueDepth uint32
	// The cumulative count of I/Os that are associated I/Os.
	SplitCount uint32
	// QueryTime: The system time when the snapshot was taken (also in 100-nanosecond increments).
	QueryTime           int64
	StorageDeviceNumber uint32
	StorageManagerName  [8]uint16
	alignmentPadding    uint32 // necessary for 32bit support, see https://github.com/elastic/beats/pull/16553
}

// Windows specific implementation of gopsutil disk.IOCountersStat,
// Has Windows specific fields
// Removed fields that are not in Windows
type IOCountersStatWindows struct {
	// Fields that are also in gopsutil disk.IOCountersStat
	Name       string `json:"name"`
	ReadCount  uint64 `json:"readCount"`
	WriteCount uint64 `json:"writeCount"`
	ReadBytes  uint64 `json:"readBytes"`
	WriteBytes uint64 `json:"writeBytes"`
	// When constructing this struct, it is converted to ms
	ReadTime float64 `json:"readTime"`
	// When constructing this struct, it is converted to ms
	WriteTime float64 `json:"writeTime"`

	// Windows Specific Fields

	// Count of the 100 ns periods the disk was idle.
	IdleTime uint64 `json:"idleTime"`

	// Count of 100 ns periods since the Win32 epoch of 01.01.1601.
	QueryTime           uint64 `json:"queryTime"`
	QueueDepth          uint32 `json:"queueDepth"`
	SplitCount          uint32 `json:"splitCount"`
	StorageDeviceNumber uint32 `json:"storageDeviceNumber"`
	StorageManagerName  string `json:"storageManagerName"`

	// Is not supported on Windows
	// MergedReadCount  uint64 `json:"mergedReadCount"`
	// MergedWriteCount uint64 `json:"mergedWriteCount"`
	// IopsInProgress   uint64 `json:"iopsInProgress"`
	// IoTime           uint64 `json:"ioTime"`
	// WeightedIO       uint64 `json:"weightedIO"`
	// SerialNumber     string `json:"serialNumber"`
	// Label            string `json:"label"`
}

// The frequency of the performance counter is fixed at system boot and is consistent across all processors.
// Therefore, the frequency need only be queried upon application initialization, and the result can be cached.
var performanceFrequency = uint64(0)

var storageDeviceNumbers map[string]storageDeviceNumberStruct

var (
	kernel32DLL                   = syscall.NewLazyDLL("kernel32.dll")
	QueryPerformanceFrequencyFunc = kernel32DLL.NewProc("QueryPerformanceFrequency")
)

func getPerformanceFrequency() (performanceFrequency uint64) {
	returnValue, _, _ := QueryPerformanceFrequencyFunc.Call(uintptr(unsafe.Pointer(&performanceFrequency)))

	if returnValue == 0 {
		log.Debugf("Could not get performance counter frequency")

		return 0
	}

	return performanceFrequency
}

func init() {
	RegisterModule(
		&AvailableTasks,
		"CheckSystem",
		"/settings/system/windows",
		NewCheckSystemHandler,
		ConfigInit{
			DefaultSystemTaskConfig,
		},
	)

	storageDeviceNumbers = getDriveStorageDeviceNumbers()

	performanceFrequency = getPerformanceFrequency()
}

// names: drive names to filter to. if empty, all drives are discovered
func IoCountersWindows(names ...string) (map[string]IOCountersStatWindows, error) {
	drivemap := make(map[string]IOCountersStatWindows, 0)
	var dPerformance diskPerformance

	// For getting a handle to the root of the drive, specify the path as \\.\PhysicalDriveX .
	// This seems to be better at picking the correct drives that can do IoctlCalls
	for deviceNumber := range uint32(32) {
		// Skip deviceNumbers that do not have a drive letter
		deviceLetter := ""
		for letter, storageDeviceNumber := range storageDeviceNumbers {
			if storageDeviceNumber.DeviceNumber == deviceNumber {
				deviceLetter = letter
			}
		}
		if deviceLetter == "" {
			continue
		}

		handlePath := `\\.\PhysicalDrive` + fmt.Sprintf("%d", deviceNumber)

		handle, err := windows.CreateFile(windows.StringToUTF16Ptr(handlePath), 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
		if err != nil {
			if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
				continue
			}

			return drivemap, fmt.Errorf("error when creating a file handle on handlePath: %s, err: %w", handlePath, err)
		}
		if handle == windows.InvalidHandle {
			continue
		}

		var diskPerformanceSize uint32
		const IOctlDiskPerformance = 0x70020
		err = windows.DeviceIoControl(handle, IOctlDiskPerformance, nil, 0, (*byte)(unsafe.Pointer(&dPerformance)), uint32(unsafe.Sizeof(dPerformance)), &diskPerformanceSize, nil)
		if err != nil {
			if errors.Is(err, windows.ERROR_INVALID_FUNCTION) {
				continue
			}
			if errors.Is(err, windows.ERROR_NOT_SUPPORTED) {
				continue
			}

			return drivemap, fmt.Errorf("error when calling IoctlDiskPerformance with a open handle to: %s, err: %w", handlePath, err)
		}

		err = windows.CloseHandle(handle)
		if err != nil {
			log.Debugf("Error when closing handle, handlePath: %s, err: %s", handlePath, err.Error())
		}

		if len(names) == 0 || slices.Contains(names, deviceLetter) {
			drivemap[deviceLetter] = IOCountersStatWindows{
				Name:       deviceLetter,
				ReadBytes:  convert.UInt64(dPerformance.BytesRead),
				WriteBytes: convert.UInt64(dPerformance.BytesWritten),
				ReadCount:  convert.UInt64(dPerformance.ReadCount),
				WriteCount: convert.UInt64(dPerformance.WriteCount),
				// these are not a total counter of time, but the period is based on performanceFrequency, a hertz value
				// 1 / performanceFrequency is the period in s, 1000 / performanceFrequency is the period in ms
				// Read time seems to be the period amount, period defined by performanceFrequency
				ReadTime: float64(dPerformance.ReadTime) * (1000 / float64(performanceFrequency)),
				// Same as ReadTime
				WriteTime:           float64(dPerformance.WriteTime) * (1000 / float64(performanceFrequency)),
				IdleTime:            convert.UInt64(dPerformance.IdleTime),  // do not convert these to ms, loses precision when calculating rate
				QueryTime:           convert.UInt64(dPerformance.QueryTime), // do not convert these to ms, loses precision when calculating rate
				QueueDepth:          convert.UInt32(dPerformance.QueueDepth),
				SplitCount:          convert.UInt32(dPerformance.SplitCount),
				StorageManagerName:  strings.TrimSpace(string(utf16.Decode(dPerformance.StorageManagerName[:]))),
				StorageDeviceNumber: convert.UInt32(dPerformance.StorageDeviceNumber),
			}
		}
	}

	return drivemap, nil
}

// This is a struct that will be filled with the Ioctl IOCTL_STORAGE_GET_DEVICE_NUMBER call.
// Does not seem to be 1:1 with storageDeviceNumberStruct field returned from Ioctl IOCTL_DISK_PERFORMANCE call for the same drive.
// https://learn.microsoft.com/en-us/windows/win32/api/winioctl/ni-winioctl-ioctl_storage_get_device_number
type storageDeviceNumberStruct struct {
	DeviceType      uint16
	DeviceNumber    uint32
	PartitionNumber uint32
}

func getDriveStorageDeviceNumbers() map[string]storageDeviceNumberStruct {
	mappings := map[string]storageDeviceNumberStruct{}

	const lpBufferLength = uint32(256)
	lpBuffer := make([]uint16, lpBufferLength)
	lpBufferLen, _ := windows.GetLogicalDriveStrings(lpBufferLength, &lpBuffer[0])

	// extract the letters out of the concatanation of multiple drive paths, e.g: "C:\D:\F:\Z:\"
	logicalDriveLetters := make([]string, 0)
	for _, wchar := range lpBuffer[:lpBufferLen] {
		if wchar < 'A' || wchar > 'Z' {
			continue
		}
		logicalDriveLetters = append(logicalDriveLetters, string(rune(wchar)))
	}

	// The buffer contains strings one after another,
	for _, logicalDriveLetter := range logicalDriveLetters {
		path := logicalDriveLetter + ":"
		handlePath := `\\.\` + path

		handle, err := windows.CreateFile(windows.StringToUTF16Ptr(handlePath), 0,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil,
			windows.OPEN_EXISTING, 0, 0)
		if err != nil {
			log.Tracef("Logical drive %s, got an error while getting a handle to %s, err: %s\n", logicalDriveLetter, handlePath, err.Error())

			continue
		}

		// Got this from Windows Kit, Microsoft does not put them in their docs for some reason.
		// C:\Program Files (x86)\Windows Kits\10\Include\10.0.26100.0\um\winioctl.h
		const IOctlStorageGetDeviceNumber = 0x2D1080
		storageDeviceNumber := storageDeviceNumberStruct{}
		bytesReturned := uint32(0)

		err = windows.DeviceIoControl(handle, IOctlStorageGetDeviceNumber, nil, 0,
			(*byte)(unsafe.Pointer(&storageDeviceNumber)), uint32(unsafe.Sizeof(storageDeviceNumber)), &bytesReturned, nil)
		if err != nil {
			log.Tracef("Logical drive %s, got an error from Ioctl IOCTL_STORAGE_GET_DEVICE_NUMBER. Likely has no physical device e.g VirtioFS/Network. Err: %s\n", logicalDriveLetter, err.Error())

			continue
		}

		err = windows.CloseHandle(handle)
		if err != nil {
			log.Debugf("Error when closing handle, handlePath: %s, err: %s", handlePath, err.Error())
		}

		mappings[logicalDriveLetter] = storageDeviceNumber
	}

	return mappings
}

// Windows uses an patched version of gopsutil disk.IOCounters() stored here
// Until the gopsutil is patched, use this version
// The function reports these attributes correctly
func (c *CheckSystemHandler) addDiskStats(create bool) {
	diskIOCounters, err := IoCountersWindows()
	// do not create the counters if there is an error
	if err != nil {
		return
	}

	if create {
		for diskName := range diskIOCounters {
			if !DiskEligibleForWatch(diskName) {
				log.Tracef("not adding disk stat counter since it is found to be not-physical: %s", diskName)

				continue
			}

			category := "disk_" + diskName
			c.snc.counterCreate(category, "write_bytes", c.bufferLength, c.metricsInterval)
			c.snc.counterCreate(category, "write_count", c.bufferLength, c.metricsInterval)
			c.snc.counterCreate(category, "read_bytes", c.bufferLength, c.metricsInterval)
			c.snc.counterCreate(category, "read_count", c.bufferLength, c.metricsInterval)
			c.snc.counterCreate(category, "idle_time", c.bufferLength, c.metricsInterval)
			c.snc.counterCreate(category, "query_time", c.bufferLength, c.metricsInterval)
		}
	}

	// use the no-copy range iteraiton, otherwise we copy the whole struct and linter does not allow it
	for diskName := range diskIOCounters {
		if !DiskEligibleForWatch(diskName) {
			continue
		}

		category := "disk_" + diskName

		c.snc.Counter.Set(category, "write_bytes", float64(diskIOCounters[diskName].WriteBytes))
		c.snc.Counter.Set(category, "write_count", float64(diskIOCounters[diskName].WriteCount))
		c.snc.Counter.Set(category, "read_bytes", float64(diskIOCounters[diskName].ReadBytes))
		c.snc.Counter.Set(category, "read_count", float64(diskIOCounters[diskName].ReadCount))
		c.snc.Counter.Set(category, "idle_time", float64(diskIOCounters[diskName].IdleTime))
		// important to put the query_time in uint64 form
		// it is some kind of nanosecond counter starting from long ago
		// even current values on 2026, the value has log2 around 56
		// float64 has 53 bits of significant precision, it loses precision
		// makes calculating utilization impossible
		c.snc.Counter.Set(category, "query_time", diskIOCounters[diskName].QueryTime)
	}
}

func (c *CheckSystemHandler) addMemoryStats(create bool) {
	if create {
		c.snc.counterCreate("memory", "total", c.bufferLength, c.metricsInterval)
		c.snc.counterCreate("memory", "used", c.bufferLength, c.metricsInterval)
		c.snc.counterCreate("memory", "swp_in", c.bufferLength, c.metricsInterval)
		c.snc.counterCreate("memory", "swp_out", c.bufferLength, c.metricsInterval)
	}

	// Windows
	// === Reports
	// Total
	// Available
	// Used
	// UsedPercent
	// Free
	// === Does Not report
	// ...
	virtualMemory, err := mem.VirtualMemory()
	if err != nil {
		return
	}

	// Windows
	// Even the reported ones do not currently work
	// === Reports
	// Total
	// Free
	// ==== Does Not report
	// ...
	// swap, err := mem.SwapMemory()
	// if err != nil {
	// 	return
	// }

	c.snc.Counter.Set("memory", "total", float64(virtualMemory.Total))
	c.snc.Counter.Set("memory", "used", float64(virtualMemory.Used))
}
