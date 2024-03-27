package snclient

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unicode"
	"unsafe"

	"pkg/humanize"
	"pkg/utils"

	"github.com/shirou/gopsutil/v3/disk"
	"golang.org/x/sys/windows"
)

const (
	IoctlStorageBase           = 0x2D
	FileDeviceMassStorage      = 0x0000002d
	MethodBuffered             = 0x0
	FileAnyAccess              = 0x0
	StorageGetHotplugInfo      = 0x0305
	IoctlStorageGetHotplugInfo = (IoctlStorageBase << 16) | (FileAnyAccess << 14) | (StorageGetHotplugInfo << 2) | MethodBuffered

	StorageGetMediaTypesEX      = 0x0301
	IoctlStorageGetMediaTypesEX = (IoctlStorageBase << 16) | (FileAnyAccess << 14) | (StorageGetMediaTypesEX << 2) | MethodBuffered
	MaxMediaTypes               = 128

	// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getvolumeinformationw
	FileReadOonlyVolume = uint32(0x00080000)
)

func (l *CheckDrivesize) getDefaultFilter() string {
	return "( mounted = 1  or media_type = 0 )"
}

func (l *CheckDrivesize) getExample() string {
	return `
    check_drivesize drive=c: show-all
    OK - c: 36.801 GiB/63.075 GiB (58.3%) |...

    check_drivesize folder=c:\Temp show-all
    OK - c: 36.801 GiB/63.075 GiB (58.3%) |...
	`
}

func (l *CheckDrivesize) getRequiredDisks(drives []string, parentFallback bool) (requiredDisks map[string]map[string]string, err error) {
	// create map of required disks/volmes with "drive_or_id" as primary key
	requiredDisks = map[string]map[string]string{}

	for _, drive := range drives {
		switch drive {
		case "*", "all":
			l.setVolumes(requiredDisks)
			err := l.setDisks(requiredDisks)
			if err != nil {
				return nil, err
			}
		case "all-drives":
			err := l.setDisks(requiredDisks)
			if err != nil {
				return nil, err
			}
		case "all-volumes":
			l.setVolumes(requiredDisks)
		default:
			// "c" or "c:"" will use the drive c
			// "c:\" will use the volume
			// "c:\path" will use the best matching volume
			l.hasCustomPath = true
			err := l.setCustomPath(drive, requiredDisks, parentFallback)
			if err != nil {
				return nil, err
			}
		}
	}

	return requiredDisks, nil
}

func (l *CheckDrivesize) addDiskDetails(ctx context.Context, check *CheckData, drive map[string]string, magic float64) {
	// set some defaults
	drive["id"] = ""
	drive["name"] = ""
	drive["media_type"] = "0"
	drive["type"] = "0"
	drive["mounted"] = "0"
	drive["readable"] = "0"
	drive["writable"] = "0"
	drive["removable"] = "0"
	drive["erasable"] = "0"
	drive["hotplug"] = ""

	if err := l.setDeviceFlags(drive); err != nil {
		log.Debugf("device flags: %s", err.Error())
	}
	if err := l.setMediaType(drive); err != nil {
		log.Debugf("device flags: %s", err.Error())
	}

	l.setDeviceInfo(drive)

	timeoutContext, cancel := context.WithTimeout(ctx, DiskDetailsTimeout)
	defer cancel()

	usage, err := disk.UsageWithContext(timeoutContext, drive["drive_or_id"])
	if err != nil {
		switch {
		case drive["type"] == "cdrom":
		case drive["removable"] != "0":
		default:
			drive["_error"] = fmt.Sprintf("failed to find disk partition %s: %s", drive["drive_or_id"], err.Error())
		}
		usage = &disk.UsageStat{}
	} else {
		drive["mounted"] = "1"
	}

	freePct := float64(0)
	if usage.Total > 0 {
		freePct = float64(usage.Free) * 100 / (float64(usage.Total))
	}

	drive["size"] = humanize.IBytesF(uint64(magic*float64(usage.Total)), 3)
	drive["size_bytes"] = fmt.Sprintf("%d", uint64(magic*float64(usage.Total)))
	drive["used"] = humanize.IBytesF(uint64(magic*float64(usage.Used)), 3)
	drive["used_bytes"] = fmt.Sprintf("%d", uint64(magic*float64(usage.Used)))
	drive["used_pct"] = fmt.Sprintf("%f", usage.UsedPercent)
	drive["free"] = humanize.IBytesF(uint64(magic*float64(usage.Free)), 3)
	drive["free_bytes"] = fmt.Sprintf("%d", uint64(magic*float64(usage.Free)))
	drive["free_pct"] = fmt.Sprintf("%f", freePct)
	drive["flags"] = strings.Join(l.getFlagNames(drive), ", ")

	l.addTotalUserMacros(drive)

	// check filter before adding metrics
	if !check.MatchMapCondition(check.filter, drive, true) {
		return
	}

	l.addMetrics(drive["drive"], check, usage, magic)
}

func (l *CheckDrivesize) setDeviceFlags(drive map[string]string) error {
	szDevice := fmt.Sprintf(`\\.\%s`, strings.TrimSuffix(drive["drive"], "\\"))
	szPtr, err := syscall.UTF16PtrFromString(szDevice)
	if err != nil {
		log.Warnf("stringPtr: %s: %s", szDevice, err.Error())

		return nil
	}
	handle, err := windows.CreateFile(szPtr, 0, 0, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		log.Tracef("create file: %s: %s", drive["letter"], err.Error())

		return nil
	}
	defer func() {
		LogDebug(windows.CloseHandle(handle))
	}()

	var num uint32
	// https://learn.microsoft.com/en-us/windows/win32/api/winioctl/ns-winioctl-storage_hotplug_info
	type storageHotplugInfo struct {
		Size                     uint32
		MediaRemovable           bool
		MediaHotplug             bool
		DeviceHotplug            bool
		WriteCacheEnableOverride bool
	}
	var hotplugInfo storageHotplugInfo
	err = windows.DeviceIoControl(
		handle,
		IoctlStorageGetHotplugInfo,
		nil,
		0,
		(*byte)(unsafe.Pointer(&hotplugInfo)),
		uint32(unsafe.Sizeof(hotplugInfo)),
		&num,
		nil,
	)
	if err != nil {
		return fmt.Errorf("deviceio %s: %s", drive["drive"], err.Error())
	}

	drive["removable"] = "0"
	if hotplugInfo.MediaRemovable {
		drive["removable"] = "1"
	}

	// seems not to be set correctly
	drive["hotplug"] = "0"
	if hotplugInfo.DeviceHotplug {
		drive["hotplug"] = "1"
	}

	return nil
}

func (l *CheckDrivesize) setMediaType(drive map[string]string) error {
	szDevice := fmt.Sprintf(`\\.\%s`, strings.TrimSuffix(drive["drive"], "\\"))
	szPtr, err := syscall.UTF16PtrFromString(szDevice)
	if err != nil {
		log.Warnf("stringPtr: %s: %s", szDevice, err.Error())

		return nil
	}
	handle, err := windows.CreateFile(szPtr, 0, 0, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		log.Tracef("create file: %s: %s", drive["letter"], err.Error())

		return nil
	}
	defer func() {
		LogDebug(windows.CloseHandle(handle))
	}()

	var num uint32
	// https://learn.microsoft.com/en-us/windows/win32/api/winioctl/ns-winioctl-get_media_types
	type getMediaTypesEX struct {
		DeviceType           uint8
		MediaInfo            uint8
		Removable            bool
		Reserved             [2]uint8
		MediaType            uint32
		MediaCharacteristics uint32
		DeviceSpecific       [8]uint8
	}

	var mediaTypesEx [MaxMediaTypes]getMediaTypesEX
	err = windows.DeviceIoControl(
		handle,
		IoctlStorageGetMediaTypesEX,
		nil,
		0,
		(*byte)(unsafe.Pointer(&mediaTypesEx)),
		uint32(unsafe.Sizeof(mediaTypesEx)),
		&num,
		nil,
	)
	if err != nil {
		return fmt.Errorf("deviceio %s: %s", drive["drive"], err.Error())
	}

	drive["media_type"] = fmt.Sprintf("%d", mediaTypesEx[0].DeviceType)

	return nil
}

func (l *CheckDrivesize) setDeviceInfo(drive map[string]string) {
	volPtr, err := syscall.UTF16PtrFromString(drive["drive_or_id"])
	if err != nil {
		log.Warnf("stringPtr: %s: %s", drive["drive_or_id"], err.Error())

		return
	}
	drive["type"] = l.driveType(windows.GetDriveType(volPtr))
	if drive["type"] == "removable" {
		drive["removable"] = "1"
	}

	volumeName := make([]uint16, 512)
	fileSystemName := make([]uint16, 512)
	fileSystemFlags := uint32(0)
	err = windows.GetVolumeInformation(
		volPtr,
		&volumeName[0],
		uint32(len(volumeName)),
		nil,
		nil,
		&fileSystemFlags,
		&fileSystemName[0],
		uint32(len(fileSystemName)))
	if err != nil {
		switch {
		case drive["type"] == "cdrom":
		case drive["removable"] != "0":
		default:
			drive["_error"] = fmt.Sprintf("cannot get volume information for %s: %s", drive["drive_or_id"], err.Error())
		}

		return
	}
	name := syscall.UTF16ToString(volumeName)
	driveOrName := drive["drive"]
	if driveOrName == "" {
		driveOrName = name
	}
	drive["readable"] = "1"
	if fileSystemFlags&FileReadOonlyVolume == 0 {
		drive["writable"] = "1"
	}

	drive["name"] = name
	if drive["fstype"] == "" {
		drive["fstype"] = syscall.UTF16ToString(fileSystemName)
	}
	drive["drive_or_name"] = driveOrName
}

func (l *CheckDrivesize) getFlagNames(drive map[string]string) []string {
	flags := []string{}
	if drive["mounted"] == "1" {
		flags = append(flags, "mounted")
	}
	if drive["hotplug"] == "1" {
		flags = append(flags, "hotplug")
	}
	if drive["readable"] == "1" {
		flags = append(flags, "readable")
	}
	if drive["writable"] == "1" {
		flags = append(flags, "writable")
	}

	return flags
}

func (l *CheckDrivesize) setDisks(requiredDisks map[string]map[string]string) (err error) {
	partitions, err := disk.Partitions(true)
	if err != nil && len(partitions) == 0 {
		// in case even a single drive is locked by BitLocker, then
		// the disk.Partitions returns an error.
		// "This drive is locked by BitLocker Drive Encryption. You must unlock this drive from Control Panel"
		// but there can still be valid elements in partitions,
		// so abort here only if partitions is empty.
		return fmt.Errorf("disk partitions failed: %s", err.Error())
	}
	for _, partition := range partitions {
		drive := partition.Device + "\\"
		entry, ok := requiredDisks[drive]
		if !ok {
			entry = make(map[string]string)
		}
		entry["drive"] = drive
		entry["drive_or_id"] = drive
		entry["drive_or_name"] = drive
		entry["letter"] = fmt.Sprintf("%c", drive[0])
		entry["fstype"] = partition.Fstype
		requiredDisks[drive] = entry
	}

	return
}

func (l *CheckDrivesize) setVolumes(requiredDisks map[string]map[string]string) {
	volumes := []string{}
	volumeName := make([]uint16, 512)
	hndl, err := windows.FindFirstVolume(&volumeName[0], uint32(len(volumeName)))
	if err != nil {
		log.Tracef("FindFirstVolume: %s", err.Error())

		return
	}
	volumes = append(volumes, syscall.UTF16ToString(volumeName))

	for {
		err := windows.FindNextVolume(hndl, &volumeName[0], uint32(len(volumeName)))
		if err != nil {
			log.Tracef("FindNextVolume: %s", err.Error())

			break
		}
		volumes = append(volumes, syscall.UTF16ToString(volumeName))
	}

	for _, vol := range volumes {
		volPtr, err := syscall.UTF16PtrFromString(vol)
		if err != nil {
			log.Warnf("stringPtr: %s: %s", vol, err.Error())

			continue
		}
		returnLen := uint32(0)
		err = windows.GetVolumePathNamesForVolumeName(volPtr, &volumeName[0], uint32(len(volumeName)), &returnLen)
		if err != nil {
			log.Warnf("GetVolumePathNamesForVolumeName: %s: %s", vol, err.Error())

			continue
		}
		names := syscall.UTF16ToString(volumeName)
		driveOrID := names
		if driveOrID == "" {
			driveOrID = vol
		}
		entry, ok := requiredDisks[driveOrID]
		if !ok {
			entry = make(map[string]string)
		}
		entry["id"] = vol
		entry["drive"] = names
		entry["drive_or_id"] = driveOrID
		entry["drive_or_name"] = names
		entry["letter"] = ""
		if names != "" {
			entry["letter"] = fmt.Sprintf("%c", names[0])
		}
		requiredDisks[driveOrID] = entry
	}
}

func (l *CheckDrivesize) setCustomPath(drive string, requiredDisks map[string]map[string]string, parentFallback bool) (err error) {
	// match a drive, ex: "c" or "c:"
	switch len(drive) {
	case 1, 2:
		drive = strings.TrimSuffix(drive, ":") + ":"
		availDisks := map[string]map[string]string{}
		err = l.setDisks(availDisks)
		for driveOrID := range availDisks {
			if strings.EqualFold(driveOrID, drive+"\\") {
				requiredDisks[drive] = utils.CloneStringMap(availDisks[driveOrID])
				requiredDisks[drive]["drive"] = drive // use name from attributes

				return nil
			}
		}
		if err != nil {
			// if setDisks had a problem (e.g. bitlocker locked drive) and did not return
			// the required drive, then pass any possible error on to the caller. otherwise
			// we got what we want and already returned nil above.
			return err
		}
	}

	// make sure path exists
	_, err = os.Stat(drive)
	if err != nil && os.IsNotExist(err) {
		log.Debugf("%s: %s", drive, err.Error())

		return fmt.Errorf("failed to find disk partition: %s", err.Error())
	}

	// try to find closes matching volume
	availVolumes := map[string]map[string]string{}
	l.setVolumes(availVolumes)
	testDrive := strings.TrimSuffix(drive, "\\")
	// make first character uppercase because drives are uppercase in the volume list
	switch {
	case len(testDrive) > 1:
		r := []rune(testDrive)
		testDrive = string(append([]rune{unicode.ToUpper(r[0])}, r[1:]...))
	case len(testDrive) == 1:
		r := []rune(testDrive)
		testDrive = string(unicode.ToUpper(r[0]))
	}
	var match *map[string]string
	for i := range availVolumes {
		vol := availVolumes[i]
		if parentFallback && vol["drive"] != "" && strings.HasPrefix(testDrive+"\\", vol["drive"]) {
			if match == nil || len((*match)["drive"]) < len(vol["drive"]) {
				match = &vol
			}
		}
		if testDrive+"\\" == vol["drive"] {
			match = &vol

			break
		}
	}
	if match != nil {
		requiredDisks[drive] = utils.CloneStringMap(*match)
		requiredDisks[drive]["drive"] = drive

		return nil
	}

	// add anyway to generate an error later with more default values filled in
	entry := l.driveEntry(drive)
	entry["_error"] = fmt.Sprintf("%s not mounted", drive)
	requiredDisks[drive] = entry

	return nil
}

func (l *CheckDrivesize) driveType(dType uint32) string {
	switch dType {
	case windows.DRIVE_UNKNOWN:
		return "unknown"
	case windows.DRIVE_NO_ROOT_DIR:
		return "no_root_dir"
	case windows.DRIVE_REMOVABLE:
		return "removable"
	case windows.DRIVE_FIXED:
		return "fixed"
	case windows.DRIVE_REMOTE:
		return "remote"
	case windows.DRIVE_CDROM:
		return "cdrom"
	case windows.DRIVE_RAMDISK:
		return "ramdisk"
	}

	return "unknown"
}
