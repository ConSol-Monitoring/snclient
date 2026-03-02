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

// All gathered storage devices
var storageDeviceNumbers map[string]storageDeviceNumberStruct

// Filtered storage devices to watch, used in disk stats
var storageDeviceNumbersToWatch map[string]storageDeviceNumberStruct

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

	storageDeviceNumbersToWatch = getStorageDeviceNumbersToWatch()

	performanceFrequency = getPerformanceFrequency()
}

// names: drive names to filter to. if empty, all drives are discovered
// tries to match physical drives only
func ioCountersWindows(names ...string) map[string]IOCountersStatWindows {
	drivemap := make(map[string]IOCountersStatWindows, 0)
	var dPerformance diskPerformance

	// For getting a handle to the root of the drive, specify the path as \\.\PhysicalDriveX .
	// This seems to be better at picking the correct drives that can do IoctlCalls
	for deviceLetter, sdn := range storageDeviceNumbersToWatch {

		handlePath := `\\.\PhysicalDrive` + fmt.Sprintf("%d", sdn.DeviceNumber)

		handle, err := windows.CreateFile(windows.StringToUTF16Ptr(handlePath), 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
		if err != nil {
			if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
				continue
			}

			log.Debugf("Error when creating a file handle on handlePath: %s, err: %s", handlePath, err.Error())
			continue
		}
		if handle == windows.InvalidHandle {
			log.Debugf("Invalid handle for PhysicalDrive %d with path: %s", sdn.DeviceNumber, handlePath)
			continue
		}

		var diskPerformanceSize uint32
		const IOctlDiskPerformance = 0x70020
		err = windows.DeviceIoControl(handle, IOctlDiskPerformance, nil, 0, (*byte)(unsafe.Pointer(&dPerformance)), uint32(unsafe.Sizeof(dPerformance)), &diskPerformanceSize, nil)
		if err != nil {
			if errors.Is(err, windows.ERROR_INVALID_FUNCTION) {
				log.Debugf("IOCTL_DISK_PERFORMANCE not supported for PhysicalDrive%d", sdn.DeviceNumber)
				errClose := windows.CloseHandle(handle)
				if errClose != nil {
					log.Debugf("Error when closing handle, handlePath: %s, err: %s", handlePath, errClose.Error())
				}
				continue
			}
			if errors.Is(err, windows.ERROR_NOT_SUPPORTED) {
				log.Debugf("IOCTL_DISK_PERFORMANCE not supported for PhysicalDrive%d", sdn.DeviceNumber)
				errClose := windows.CloseHandle(handle)
				if errClose != nil {
					log.Debugf("Error when closing handle, handlePath: %s, err: %s", handlePath, errClose.Error())
				}
				continue
			}

			log.Debugf("Error when calling IoctlDiskPerformance with a open handle to: %s, err: %s", handlePath, err.Error())
			errClose := windows.CloseHandle(handle)
			if errClose != nil {
				log.Debugf("Error when closing handle, handlePath: %s, err: %s", handlePath, errClose.Error())
			}
			continue
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

	return drivemap
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
			// Close handle before continuing
			errClose := windows.CloseHandle(handle)
			if errClose != nil {
				log.Debugf("Error when closing handle, handlePath: %s, err: %s", handlePath, errClose.Error())
			}
			continue
		}

		err = windows.CloseHandle(handle)
		if err != nil {
			log.Debugf("Error when closing handle, handlePath: %s, err: %s", handlePath, err.Error())
		}

		mappings[logicalDriveLetter] = storageDeviceNumber
		log.Debugf("Adding to storageDeviceNumber map. Letter: %s, Device Number: %d, DeviceType: %d, Partition Number: %d", logicalDriveLetter, storageDeviceNumber.DeviceNumber, storageDeviceNumber.DeviceType, storageDeviceNumber.PartitionNumber)
	}

	return mappings
}

func getStorageDeviceNumbersToWatch() map[string]storageDeviceNumberStruct {
	// filter the real physical drives from all gathered storageDeviceNumbers
	storageDeviceNumbersToWatch := make(map[string]storageDeviceNumberStruct)

	// first 32 devices are more likely to be real physical drives
	for deviceNumber := range uint32(32) {
		// multiple letters might share the same storage device if disk is partitioned
		for letter, sdn := range storageDeviceNumbers {
			if sdn.DeviceNumber != deviceNumber {
				continue
			}

			// seems to be reserved for non-physical drives
			// for example a CD drive looked like this:
			// storageDeviceNumber.DeviceNumber = 0 and storageDeviceNumber.PartitionNumber = 4294967295
			if sdn.PartitionNumber > 32 {
				log.Tracef("Device unfit to watch, is likely not a drive due to high partitionNumber. Letter: %s, DeviceNumber: %d, DeviceType: %d, Partition Number: %d", letter, sdn.DeviceNumber, sdn.DeviceType, sdn.PartitionNumber)
				continue
			}

			// C:\Program Files (x86)\Windows Kits\10\Include\10.0.26100.0\um\winioctl.h
			// FILE_DEVICE_DISK = 7
			if sdn.DeviceType != 7 {
				log.Tracef("Device unfit to watch, its deviceType is not a disk. Letter: %s, DeviceNumber: %d, DeviceType: %d, Partition Number: %d", letter, sdn.DeviceNumber, sdn.DeviceType, sdn.PartitionNumber)
				continue
			}

			log.Debugf("Adding to storageDeviceNumbersToWatch. Letter: %s, DeviceNumber: %d, DeviceType: %d, Partition Number: %d", letter, sdn.DeviceNumber, sdn.DeviceType, sdn.PartitionNumber)
			storageDeviceNumbersToWatch[letter] = sdn
		}
	}

	return storageDeviceNumbersToWatch
}

// Windows uses an patched version of gopsutil disk.IOCounters() stored here
// Until the gopsutil is patched, use this version
// The function reports these attributes correctly
func (c *CheckSystemHandler) addDiskStats(create bool) {
	diskIOCounters := ioCountersWindows()
	// do not create the counters if there is an error

	if create {
		for diskName := range diskIOCounters {
			if !diskEligibleForWatch(diskName) {
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
		if !diskEligibleForWatch(diskName) {
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
}
