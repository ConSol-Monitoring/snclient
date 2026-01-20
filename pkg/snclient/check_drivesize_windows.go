package snclient

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unicode"
	"unsafe"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/utils"
	"github.com/shirou/gopsutil/v4/disk"
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
	volumeOptReadOnly = uint32(0x00080000)
	volumeCompressed  = uint32(0x00008000)
)

func (l *CheckDrivesize) getDefaultFilter() string {
	return "mounted = 1"
}

func (l *CheckDrivesize) getExample() string {
	return `
    check_drivesize drive=c: show-all
    OK - c: 36.801 GiB/63.075 GiB (58.3%) |...

    check_drivesize folder=c:\Temp show-all
    OK - c: 36.801 GiB/63.075 GiB (58.3%) |...
	`
}

// Terminology
// Disk : Physical Hardware like a HDD, SSD, Usb Stick. A block device that ca be used to store raw bytes -> \\.\PhysicalDrive0
// Partition: Is written into the disk in a partition table. It exists independently of volumes, may not be used by Windows
// Volume: A logical abstraction of a storage, formatted with a file system. It can be a virtual file, a RAID disk or one partition -> \\?\Volume{GUID}\
// Drive: This term is not defined well. Here, it means a logical access point with an assigned drive letter.

// For a given path, it may correspond to a mounted disk, like a \\SERVER\SHARENAME being added as K: drive
// Checks the given path, resolves the alias and calls the corresponding function that adds the details
func (l *CheckDrivesize) getRequiredDrives(paths []string, parentFallback bool) (requiredDrives map[string]map[string]string, err error) {
	// create map of required disks/volmes/network_shares with "drive_or_id" as primary key

	requiredDrives = map[string]map[string]string{}

	// if there are multiple drive= arguments, these functions may be called multiple times.
	// they check if key exists before adding it to requiredDrives.
	for _, drive := range paths {
		switch drive {
		case "*", "all":
			l.setVolumes(requiredDrives)
			err := l.setDrives(requiredDrives)
			if err != nil {
				return nil, err
			}
			l.setShares(requiredDrives)
		case "all-drives":
			err := l.setDrives(requiredDrives)
			if err != nil {
				return nil, err
			}
		case "all-volumes":
			l.setVolumes(requiredDrives)
		case "all-shares":
			l.setShares(requiredDrives)
		default:
			l.hasCustomPath = true
			err := l.setCustomPath(drive, requiredDrives, parentFallback)
			if err != nil {
				return nil, err
			}
		}
	}

	return requiredDrives, nil
}

// this is the main function where most of the attributes are added
// it also adds the capacity and usage metrics
func (l *CheckDrivesize) addDiskDetails(ctx context.Context, check *CheckData, drive map[string]string, magic float64) {
	// check filter before querying disk
	if !check.MatchMapCondition(check.filter, drive, true) {
		return
	}

	// set some defaults
	drive["id"] = ""
	drive["name"] = ""
	drive["media_type"] = "0"
	drive["type"] = "0"
	drive["readable"] = "0"
	drive["writable"] = "0"
	drive["removable"] = "0"
	drive["erasable"] = "0"
	drive["hotplug"] = ""

	l.setDeviceInfo(drive)

	if drive["type"] != "remote" {
		if err := l.setDeviceFlags(drive); err != nil {
			log.Debugf("device flags: %s", err.Error())
		}
		if err := l.setMediaType(drive); err != nil {
			log.Debugf("device flags: %s", err.Error())
		}
	}

	timeoutContext, cancel := context.WithTimeout(ctx, DiskDetailsTimeout)
	defer cancel()

	// Uses gopsutil to check disk usage
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
		if _, ok := drive["mounted"]; !ok {
			drive["mounted"] = "1"
		}
	}

	if _, ok := drive["mounted"]; !ok {
		drive["mounted"] = "0"
	}

	if check != nil {
		l.addDriveSizeDetails(check, drive, usage, magic)
	}
}

// Creates a handle to the root of the drive, and calls StorageHotplugInfo Ioctl
// This will not work on remote drives
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

// Creates a handle at the root of the drive, and calls GetMediaTypes Ioctl
// This will not work on remote drives
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

	drive["media_type"] = fmt.Sprintf("%d", mediaTypesEx[0].MediaType)

	return nil
}

// uses GetVolumeInformaton syscall, works on network drives
//
//nolint:funlen //no need to split this
func (l *CheckDrivesize) setDeviceInfo(drive map[string]string) {
	driveType, err := GetDriveType(drive["drive_or_id"])
	if err != nil {
		log.Warnf("Error when getting the drive type of drive %s: %s", drive["drive_or_id"], err.Error())

		return
	}
	drive["type"] = driveType.toString()
	if drive["type"] == "removable" {
		drive["removable"] = "1"
	}

	driveUTF16, err := syscall.UTF16PtrFromString(drive["drive_or_id"])
	if err != nil {
		log.Warnf("Cannot convert drive to UTF16 : %s: %s", drive["drive_or_id"], err.Error())

		return
	}
	volumeName := make([]uint16, windows.MAX_PATH+1)
	volumeNameLen, err := convert.UInt32E(len(volumeName))
	if err != nil {
		drive["_error"] = fmt.Sprintf("cannot set length of volume name: %s", err.Error())

		return
	}
	fileSystemName := make([]uint16, windows.MAX_PATH+1)
	fileSystemNameLen, err := convert.UInt32E(len(fileSystemName))
	if err != nil {
		drive["_error"] = fmt.Sprintf("cannot set length of filesystem name: %s", err.Error())

		return
	}
	fileSystemFlags := uint32(0)
	opts := []string{}
	// uses GetVolumeInformationW
	// This system call works on network drives
	err = windows.GetVolumeInformation(
		driveUTF16,
		&volumeName[0],
		volumeNameLen,
		nil,
		nil,
		&fileSystemFlags,
		&fileSystemName[0],
		fileSystemNameLen)
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
	if fileSystemFlags&volumeOptReadOnly == 0 {
		drive["writable"] = "1"
		opts = append(opts, "rw")
	} else {
		opts = append(opts, "ro")
	}
	if fileSystemFlags&volumeCompressed == 0 {
		opts = append(opts, "compress")
	}

	drive["opts"] = strings.Join(opts, ",")
	drive["name"] = name
	if drive["fstype"] == "" {
		drive["fstype"] = syscall.UTF16ToString(fileSystemName)
	}
	if drive["drive_or_name"] == "" {
		drive["drive_or_name"] = driveOrName
	}
}

// adds all logical drives to the requiredDisks
// this function is better suited for usage with-network drives. gopsutil has disk.Partitions, but that does not work with network drives
func (l *CheckDrivesize) setDrives(requiredDrives map[string]map[string]string) (err error) {
	logicalDrives, err := GetLogicalDriveStrings(1024)
	if err != nil {
		log.Debug("Error when getting logical drive strings: %s", err.Error())
	}

	for _, logicalDrive := range logicalDrives {
		entry, ok := requiredDrives[logicalDrive]
		if !ok {
			entry = make(map[string]string)
		}
		entry["drive"] = logicalDrive
		entry["drive_or_id"] = logicalDrive
		entry["drive_or_name"] = logicalDrive
		entry["letter"] = fmt.Sprintf("%c", logicalDrive[0])
		requiredDrives[logicalDrive] = entry
	}

	return nil
}

// adds all logical volumes to the requiredDisks
// network shares are not listed in volumes
func (l *CheckDrivesize) setVolumes(requiredDrives map[string]map[string]string) {
	volumeGUIDPaths := []string{}
	bufLen, err := convert.UInt32E(windows.MAX_PATH + 1)
	if err != nil {
		log.Tracef("convert.UInt32E: %s", err.Error())

		return
	}

	volumeGUIDPathBuffer := make([]uint16, bufLen)
	hndl, err := windows.FindFirstVolume(&volumeGUIDPathBuffer[0], bufLen)
	if err != nil {
		log.Tracef("FindFirstVolume: %s", err.Error())

		return
	}
	defer func() {
		LogDebug(windows.FindVolumeClose(hndl))
	}()

	volumeGUIDPaths = append(volumeGUIDPaths, syscall.UTF16ToString(volumeGUIDPathBuffer))

	for {
		err := windows.FindNextVolume(hndl, &volumeGUIDPathBuffer[0], bufLen)
		if err != nil {
			log.Tracef("FindNextVolume: %s", err.Error())

			break
		}
		volumeGUIDPaths = append(volumeGUIDPaths, syscall.UTF16ToString(volumeGUIDPathBuffer))
	}

	// Windows syscall findFirstVolume, findNextVolume... give GUID paths to volumes e.g:
	// "\\\\?\\Volume{a6b8f57e-dac6-4bac-8dc2-fac22cd740cf}\\"
	// They need to be translated
	for _, volumeGUIDPath := range volumeGUIDPaths {
		// reuse the buffer when filling the details
		l.setVolume(requiredDrives, volumeGUIDPath, volumeGUIDPathBuffer)
	}
}

// this function is used to further process the volume GUID path returned from API call
// takes a GUID path of a volume, finds its path name. it may or may not be mounted directly on a drive letter
func (l *CheckDrivesize) setVolume(requiredDisks map[string]map[string]string, volumeGUIDPath string, buffer []uint16) {
	volPtr, err := syscall.UTF16PtrFromString(volumeGUIDPath)
	if err != nil {
		log.Warnf("stringPtr: %s: %s", volumeGUIDPath, err.Error())

		return
	}
	returnLen := uint32(0)
	bufferLen, err := convert.UInt32E(len(buffer))
	if err != nil {
		log.Warnf("convert.UInt32E: %s", err.Error())

		return
	}
	// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getvolumepathnamesforvolumenamew
	// SMB does not support volume management functions
	// uses GetVolumePathNamesForVolumeNameW
	err = windows.GetVolumePathNamesForVolumeName(volPtr, &buffer[0], bufferLen, &returnLen)
	if err != nil {
		log.Warnf("Error when calling GetVolumePathNamesForVolumeName: %s: %s", volumeGUIDPath, err.Error())

		return
	}
	names := syscall.UTF16ToString(buffer)
	driveOrID := names
	if driveOrID == "" {
		driveOrID = volumeGUIDPath
	}
	// only add it if it does not exists
	entry, ok := requiredDisks[driveOrID]
	if !ok {
		entry = make(map[string]string)
	}
	entry["id"] = volumeGUIDPath
	entry["drive"] = names
	entry["drive_or_id"] = driveOrID
	entry["drive_or_name"] = names
	entry["letter"] = ""
	if names != "" {
		entry["letter"] = fmt.Sprintf("%c", names[0])
	} else {
		entry["mounted"] = "0"
	}
	requiredDisks[driveOrID] = entry
}

// This function is called when a custom path needs to be added
// This is used if folders are given
// c:/, d:/volume, f:/folder/with/slash
//
//nolint:funlen // can not split this function up
func (l *CheckDrivesize) setCustomPath(path string, requiredDisks map[string]map[string]string, parentFallback bool) (err error) {
	path = strings.ReplaceAll(path, "/", "\\")
	// if its a network share path, discover existing shares and match it with a drive[remote_path]
	// then we replace path argument in-place, replacing the network path with the logical drive it is assigned to
	if isNetworkSharePath(path) {
		discoveredNetworkShares := map[string]map[string]string{}
		l.setShares(discoveredNetworkShares)

		for key := range discoveredNetworkShares {
			drive := discoveredNetworkShares[key]
			remoteName, hasRemoteName := drive["remote_name"]
			if hasRemoteName && strings.HasPrefix(path, remoteName) {
				requiredDisks[key] = utils.CloneStringMap(discoveredNetworkShares[key])
				// drive["remote_name"] = \\SERVER\SHARENAME
				// drive["drive"] = X:\
				// pathExample1 = \\SERVER\SHARENAME -> X:\
				// pathExample2 = \\SERVER\SHARENAME\FOO\BAR -> X:\FOO\BAR
				pathReplaced := strings.Replace(path, drive["remote_name"], drive["drive"], 1)
				requiredDisks[key]["drive"] = pathReplaced
				// It is better to let users set their own detailSyntax or okSyntax, we give them the attributes for it
				// requiredDisks[key]["drive_or_name"] = fmt.Sprintf("%s - (%s)", path, pathReplaced)
				requiredDisks[key]["localised_remote_path"] = pathReplaced

				return nil
			}
		}
	}

	// match a drive, ex: "c" or "c:"
	switch len(path) {
	case 1, 2:
		path = strings.TrimSuffix(path, ":") + ":"
		availDisks := map[string]map[string]string{}
		err = l.setDrives(availDisks)
		for driveOrID := range availDisks {
			if strings.EqualFold(driveOrID, path+"\\") {
				requiredDisks[path] = utils.CloneStringMap(availDisks[driveOrID])
				requiredDisks[path]["drive"] = path // use name from attributes

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
	_, err = os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		log.Debugf("%s: %s", path, err.Error())

		return &PartitionNotFoundError{Path: path, err: err}
	}

	// try to find closes matching volume
	availVolumes := map[string]map[string]string{}
	l.setVolumes(availVolumes)
	testDrive := strings.TrimSuffix(path, "\\")
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
		if parentFallback && vol["drive"] != "" && strings.HasPrefix(strings.ToUpper(testDrive+"\\"), strings.ToUpper(vol["drive"])) {
			if match == nil || len((*match)["drive"]) < len(vol["drive"]) {
				match = &vol
			}
		}
		if strings.EqualFold(testDrive+"\\", vol["drive"]) {
			match = &vol

			break
		}
	}
	if match != nil {
		requiredDisks[path] = utils.CloneStringMap(*match)
		requiredDisks[path]["drive"] = path

		return nil
	}

	// add anyway to generate an error later with more default values filled in
	entry := l.driveEntry(path)
	entry["_error"] = fmt.Sprintf("%s not mounted", path)
	requiredDisks[path] = entry

	return nil
}

// adds all network shares to requiredDisks
func (l *CheckDrivesize) setShares(requiredDisks map[string]map[string]string) {
	logicalDrives, err := GetLogicalDriveStrings(1024)
	if err != nil {
		log.Debug("Error when getting logical drive strings: %s", err.Error())
	}

	for _, logicalDrive := range logicalDrives {
		driveType, err := GetDriveType(logicalDrive)
		if err != nil {
			log.Debug("Error when getting the drive type for logical drive %s network drives: %s", logicalDrive, err.Error())

			continue
		}
		if driveType == DriveRemote {
			remoteName, err := NetGetConnection(logicalDrive[0 : len(logicalDrive)-1])
			if err != nil {
				log.Debug("Error when getting the connection for logical drive %s : %s", logicalDrive, err.Error())

				continue
			}
			log.Debugf("Logical Drive: %s, Drive Type: %d, Remote name: %s", logicalDrive, driveType, remoteName)
			// modify existing drive if its there
			// if its not, add it new
			drive, ok := requiredDisks[logicalDrive]
			if !ok {
				drive = make(map[string]string)
			}
			drive["id"] = remoteName
			drive["drive"] = logicalDrive
			drive["drive_or_id"] = logicalDrive
			// It is better to let users set their own detailSyntax or okSyntax, we give them the attributes for it
			drive["drive_or_name"] = logicalDrive
			drive["letter"] = logicalDrive
			drive["remote_name"] = remoteName
			if isNetworkDrivePersistent(logicalDrive) {
				drive["persistent"] = "1"
			} else {
				drive["persistent"] = "0"
			}
			requiredDisks[logicalDrive] = drive
		}
	}
}

// driveLetter is assumed to be like 'X:\'
func isNetworkDrivePersistent(driveLetter string) (isPersistent bool) {
	driveLetter = driveLetter[0:1]
	persistentNetworkDrives, err := discoverPersistentNetworkDrives()
	if err != nil {
		log.Debug("Error when discovering persistent network drives: %s", err.Error())
	}
	for _, drive := range persistentNetworkDrives {
		if drive.DriveLetter == driveLetter {
			return true
		}
	}
	log.Debugf("Found no persistent network drive with driveLetter %s", driveLetter)

	return false
}

// returns if the path looks like an UNC path
func isNetworkSharePath(path string) (isNetworkSharePath bool) {
	// Example 1: \\FileServer01\PublicDocs
	// Example 2: \\BackupServer\Data\Archive\2025-01-14.zip
	// Example 3: \\192.168.1.50\SharedData\Images|
	// But modern programs also generally accept forward slash definitions
	// //192.168.1.50/Shareddata/Images

	if len(path) < 2 {
		return false
	}

	if !strings.HasPrefix(path, "\\\\") && !strings.HasPrefix(path, "//") {
		return false
	}

	return true
}
