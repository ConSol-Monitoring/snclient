package snclient

import (
	"errors"
	"fmt"
	"slices"
	"unsafe"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"golang.org/x/sys/windows"
)

// diskPerformance is an equivalent representation of DISK_PERFORMANCE in the Windows API.
// https://docs.microsoft.com/fr-fr/windows/win32/api/winioctl/ns-winioctl-disk_performance
type diskPerformanceCustom struct {
	BytesRead           int64
	BytesWritten        int64
	ReadTime            int64
	WriteTime           int64
	IdleTime            int64
	ReadCount           uint32
	WriteCount          uint32
	QueueDepth          uint32
	SplitCount          uint32
	QueryTime           int64
	StorageDeviceNumber uint32
	StorageManagerName  [8]uint16
	alignmentPadding    uint32 // necessary for 32bit support, see https://github.com/elastic/beats/pull/16553
}

var storageDeviceNumbers map[string]StorageDeviceNumber

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
}

// names: drive names to filter to. if empty, all drives are discovered
func IoCountersCustom(names ...string) (map[string]disk.IOCountersStat, error) {
	drivemap := make(map[string]disk.IOCountersStat, 0)
	var dPerformance diskPerformanceCustom

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

		const IOCTL_DISK_PERFORMANCE = 0x70020
		h, err := windows.CreateFile(windows.StringToUTF16Ptr(handlePath), 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
		if err != nil {
			if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
				continue
			}
			return drivemap, err
		}
		if h == windows.InvalidHandle {
			continue
		}
		defer windows.CloseHandle(h)

		var diskPerformanceSize uint32
		err = windows.DeviceIoControl(h, IOCTL_DISK_PERFORMANCE, nil, 0, (*byte)(unsafe.Pointer(&dPerformance)), uint32(unsafe.Sizeof(dPerformance)), &diskPerformanceSize, nil)
		if err != nil {
			if errors.Is(err, windows.ERROR_INVALID_FUNCTION) {
				continue
			}
			if errors.Is(err, windows.ERROR_NOT_SUPPORTED) {
				continue
			}
			return drivemap, err
		}

		if len(names) == 0 || slices.Contains(names, deviceLetter) {
			drivemap[deviceLetter] = disk.IOCountersStat{
				ReadBytes:  uint64(dPerformance.BytesRead),
				WriteBytes: uint64(dPerformance.BytesWritten),
				ReadCount:  uint64(dPerformance.ReadCount),
				WriteCount: uint64(dPerformance.WriteCount),
				ReadTime:   uint64(dPerformance.ReadTime / 10000 / 1000), // convert to ms: https://github.com/giampaolo/psutil/issues/1012
				WriteTime:  uint64(dPerformance.WriteTime / 10000 / 1000),
				Name:       deviceLetter,
			}
		}
	}
	return drivemap, nil
}

// Got this from Windows Kit
// C:\Program Files (x86)\Windows Kits\10\Include\10.0.26100.0\um\winioctl.h
const IOCTL_STORAGE_GET_DEVICE_NUMBER = 0x2D1080

// This is a struct that will be filled with the Ioctl call.
// https://learn.microsoft.com/en-us/windows/win32/api/winioctl/ni-winioctl-ioctl_storage_get_device_number
type StorageDeviceNumber struct {
	DeviceType      uint16
	DeviceNumber    uint32
	PartitionNumber uint32
}

func getDriveStorageDeviceNumbers() map[string]StorageDeviceNumber {
	mappings := map[string]StorageDeviceNumber{}

	lpBuffer := make([]uint16, 254)
	lpBufferLen, _ := windows.GetLogicalDriveStrings(uint32(len(lpBuffer)), &lpBuffer[0])

	// extract the letters out of the concatanation of multiple drive paths, e.g: C:\D:\F:\Z:\
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
		szDevice := `\\.\` + path

		handle, err := windows.CreateFile(windows.StringToUTF16Ptr(szDevice), 0,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil,
			windows.OPEN_EXISTING, 0, 0)
		if err != nil {
			log.Tracef("Logical drive %s, got an error while getting a handle to %s, err: %s\n", logicalDriveLetter, szDevice, err.Error())
			continue
		}
		defer windows.CloseHandle(handle)

		storageDeviceNumber := StorageDeviceNumber{}
		bytesReturned := uint32(0)

		err = windows.DeviceIoControl(handle, IOCTL_STORAGE_GET_DEVICE_NUMBER, nil, 0,
			(*byte)(unsafe.Pointer(&storageDeviceNumber)), uint32(unsafe.Sizeof(storageDeviceNumber)), &bytesReturned, nil)

		if err != nil {
			log.Tracef("Logical drive %s, got an error from Ioctl IOCTL_STORAGE_GET_DEVICE_NUMBER. Likely has no physical device e.g VirtioFS/Network. Err: %s\n", logicalDriveLetter, err.Error())
			continue
		}

		mappings[logicalDriveLetter] = storageDeviceNumber
	}

	return mappings
}

// Windows uses an patched version of gopsutil disk.IOCounters() stored here
// Until the gopsutil is patched, use this version
// The function reports these attributes correctly
func (c *CheckSystemHandler) addDiskStats(create bool) {
	diskIOCounters, err := IoCountersCustom()
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
