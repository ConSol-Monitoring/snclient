package snclient

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

// For a given path, it may correspond to a mounted disk, like a normal C:\ drive
// Or it could be a network drive like \\SERVER\SHARENAME being mounted to K:\ drive
// Checks the given path, resolves the alias and calls the corresponding function that adds the details
func (l *CheckDrivesize) getRequiredDrives(paths []string, parentFallback bool) (requiredDrives map[string]map[string]string, err error) {
	// create map of required disks/volmes/network_shares
	// this map will be populated piece by piece
	requiredDrives = map[string]map[string]string{}

	// if there are multiple drive= arguments, these functions may be called multiple times.
	// they check if key exists before adding it to requiredDrives.
	for _, drive := range paths {
		switch drive {
		case "*", "all":
			err := l.setDrives(requiredDrives)
			if err != nil {
				return nil, err
			}
			l.setVolumes(requiredDrives)
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
		log.Warnf("Error when getting the drive type of drive, drive: '%s' , error: %s", drive["drive_or_id"], err.Error())

		return
	}
	drive["type"] = driveType.toString()
	if drive["type"] == "removable" {
		drive["removable"] = "1"
	}

	// drivePath needs to be in form 'X:\'
	drivePath := strings.ToUpper(drive["drive_or_id"])
	if !strings.HasSuffix(drivePath, "\\") {
		drivePath += "\\"
	}
	driveUTF16, err := syscall.UTF16PtrFromString(drivePath)
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
		fileSystemNameLen,
	)
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
	drive["name"] = name

	driveOrName := drive["drive"]
	if driveOrName == "" {
		driveOrName = name
	}
	if drive["drive_or_name"] == "" {
		drive["drive_or_name"] = driveOrName
	}

	driveOrNameOrID := driveOrName
	if driveOrNameOrID == "" {
		driveOrNameOrID = drive["drive_or_id"]
	}
	if drive["drive_or_name_or_id"] == "" {
		drive["drive_or_name_or_id"] = driveOrNameOrID
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

	if drive["fstype"] == "" {
		drive["fstype"] = syscall.UTF16ToString(fileSystemName)
	}
}

// gopsutil disk.Partition had an issue with Bitlocker, but a fix was upstreamed
func (l *CheckDrivesize) setDrives(requiredDrives map[string]map[string]string) (err error) {
	partitions, err := disk.Partitions(true)
	if err != nil && len(partitions) == 0 {
		return fmt.Errorf("disk partitions failed: %s", err.Error())
	}
	for _, partition := range partitions {
		// skip empty partitions
		if partition.Device == "" && partition.Mountpoint == "" && partition.Fstype == "" {
			continue
		}
		logicalDrive := strings.TrimSuffix(partition.Device, "\\") + "\\"
		entry, ok := requiredDrives[logicalDrive]
		if !ok {
			entry = make(map[string]string)
		}
		entry["drive"] = logicalDrive
		entry["name"] = logicalDrive
		entry["drive_or_id"] = logicalDrive
		entry["drive_or_name"] = logicalDrive
		entry["drive_or_name_or_id"] = logicalDrive
		entry["letter"] = fmt.Sprintf("%c", logicalDrive[0])
		entry["fstype"] = partition.Fstype
		requiredDrives[logicalDrive] = entry
	}

	return nil
}

// adds all logical volumes to the requiredDrives
// network shares are not listed in volumes
// this function populates requiredDrives map
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
			log.Tracef("FindNextVolume error, likely iterated every volume, error: %s", err.Error())

			break
		}
		volumeGUIDPaths = append(volumeGUIDPaths, syscall.UTF16ToString(volumeGUIDPathBuffer))
	}

	log.Tracef("Found volume GUID paths: %s", strings.Join(volumeGUIDPaths, ", "))

	// Windows syscall findFirstVolume, findNextVolume... give GUID paths to volumes e.g:
	// "\\\\?\\Volume{a6b8f57e-dac6-4bac-8dc2-fac22cd740cf}\\"
	// They need to be translated
	for _, volumeGUIDPath := range volumeGUIDPaths {
		// reuse the buffer when filling the details
		l.setVolume(requiredDrives, volumeGUIDPath, volumeGUIDPathBuffer)
	}
}

// this function is used to further process the volume GUID path returned from Windows API calls
// takes a GUID path of a volume, then finds its path name.
// it may or may not be mounted directly on a drive letter
//
//nolint:funlen // there are a lot of entry attributes
func (l *CheckDrivesize) setVolume(requiredDrives map[string]map[string]string, volumeGUIDPath string, buffer []uint16) {
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
		log.Warnf("Error when calling GetVolumePathNamesForVolumeName, volumeGUIDPath: '%s' , error: %s", volumeGUIDPath, err.Error())

		return
	}
	volumePathName := syscall.UTF16ToString(buffer)

	if volumePathName == "" {
		log.Tracef("volume has no path name, using GUID path as path: %s", volumeGUIDPath)
		volumePathName = volumeGUIDPath
	}

	// entry attributes
	volumeID := volumeGUIDPath

	name := volumePathName

	drive, isDrive, _ := cleanupPathString(volumePathName)
	if !isDrive {
		drive = ""
	}

	driveOrID := drive
	if driveOrID == "" {
		driveOrID = volumeID
	}

	driveOrName := drive
	if driveOrName == "" {
		driveOrName = name
	}

	driveOrNameOrID := driveOrName
	if driveOrNameOrID == "" {
		driveOrNameOrID = volumeID
	}

	letter := ""
	if drive != "" {
		letter = fmt.Sprintf("%c", drive[0])
	}

	mounted := "0"
	if letter != "" {
		mounted = "1"
	}

	for _, existingDrive := range requiredDrives {
		if existingDrive["drive"] == drive {
			// drive already added
			return
		}
	}

	// check if there exists an entry
	entry, idAlreadyAdded := requiredDrives[volumeID]
	if !idAlreadyAdded {
		entry = make(map[string]string)
	}

	entry["id"] = volumeID
	entry["drive"] = drive
	entry["name"] = name
	entry["drive_or_id"] = driveOrID
	entry["drive_or_name"] = driveOrName
	entry["drive_or_name_or_id"] = driveOrNameOrID
	entry["letter"] = letter
	entry["mounted"] = mounted

	requiredDrives[volumeID] = entry
}

// The perflabel prefix situation it complicated due to compatibility reasons
func getPerflabelPrefix(path string) (perflabelPrefix string, err error) {
	if path == "" {
		return "", fmt.Errorf("path to get perflabel for is empty")
	}

	perflabelPrefix = filepath.FromSlash(path)

	sep := string(os.PathSeparator)
	for strings.Contains(perflabelPrefix, sep+sep) {
		perflabelPrefix = strings.ReplaceAll(perflabelPrefix, sep+sep, sep)
	}

	if len(perflabelPrefix) == 1 {
		perflabelPrefix += ":"
	}

	return perflabelPrefix, nil
}

// cleans up an absolute path
// changes all slashes to be backwards
// compacts slashes that come one after another
// changes the drive letter to be uppercase, if present
// adds a colon after the drive letter if its not present
// adds a backward slash after colon if not present
func cleanupPathString(path string) (cleanedPath string, isDrive bool, err error) {
	if path == "" {
		return "", false, fmt.Errorf("path to cleanup is empty")
	}

	cleanedPath = filepath.FromSlash(path)
	isDrive = true

	sep := string(os.PathSeparator)
	for strings.Contains(cleanedPath, sep+sep) {
		cleanedPath = strings.ReplaceAll(cleanedPath, sep+sep, sep)
	}

	if len(cleanedPath) >= 1 && !unicode.IsLetter(rune(path[0])) {
		log.Tracef("Cleaned paths first rune is not a letter, cannot be a drive, path: '%s' ", path)
		isDrive = false
	}

	if len(cleanedPath) >= 1 && unicode.IsLetter(rune(path[0])) {
		cleanedPath = strings.ToUpper(string(cleanedPath[0])) + cleanedPath[1:]
	}

	if len(cleanedPath) == 1 {
		cleanedPath += ":"
	}

	if len(cleanedPath) >= 2 && cleanedPath[1] != ':' {
		log.Tracef("Cleaned paths second rune is not colon, cannot be a drive, path: '%s' ", path)
		isDrive = false
	}

	if len(cleanedPath) == 2 {
		cleanedPath += "\\"
	}

	if len(cleanedPath) >= 3 && cleanedPath[2] != '\\' {
		log.Tracef("Cleaned paths third rune is not backslash, cannot be a drive, path: '%s' ", path)
		isDrive = false
	}

	if len(cleanedPath) != 3 || (len(cleanedPath) >= 1 && !unicode.IsLetter(rune(path[0]))) ||
		(len(cleanedPath) >= 2 && cleanedPath[1] != ':') || (len(cleanedPath) >= 3 && cleanedPath[2] != '\\') {
		isDrive = false
	}

	return cleanedPath, isDrive, nil
}

// This function is called when a custom path needs to be added
// This is used if folders are given
// c:/, d:/volume, f:/folder/with/slash
//
//nolint:funlen // can not split this function up, it has to check if its network drive, normal drive, a custom path under a volume etc.
func (l *CheckDrivesize) setCustomPath(path string, requiredDrives map[string]map[string]string, parentFallback bool) (err error) {
	// --------- Option 1 : Network share path

	// if its a network share path, discover existing shares and match it with a drive[remote_path]
	// then we replace path argument in-place, replacing the network path with the logical drive it is assigned to
	if isNetworkSharePath(path) {
		discoveredNetworkShares := map[string]map[string]string{}
		l.setShares(discoveredNetworkShares)

		for key := range discoveredNetworkShares {
			networkShare := discoveredNetworkShares[key]
			remoteName, hasRemoteName := networkShare["remote_name"]
			if hasRemoteName && strings.HasPrefix(path, remoteName) {
				requiredDrives[key] = utils.CloneStringMap(discoveredNetworkShares[key])

				// drive["remote_name"] = \\SERVER\SHARENAME
				// drive["drive"] = x:
				// pathExample1 = \\SERVER\SHARENAME -> x:
				// pathExample2 = \\SERVER\SHARENAME\FOO\BAR -> x:\FOO\BAR
				pathReplaced := strings.Replace(path, networkShare["remote_name"], networkShare["drive"], 1)
				// It is better to let users set their own detailSyntax or okSyntax, we give them the attributes for it
				// requiredDrives[key]["drive_or_name"] = fmt.Sprintf("%s - (%s)", path, pathReplaced)
				requiredDrives[key]["localised_remote_path"] = pathReplaced

				return nil
			}
		}
	}

	// Important: UNC network paths have slashes next to each other e.g: \\ServerName\SharedFolder\ResourcePath
	// This gets cleaned up using cleanupPathString , as it is meant for absolute paths inside a drive.
	// Not cleaning up the path beforehand is intentional
	cleanedPath, isDrivePath, err := cleanupPathString(path)
	if err != nil {
		return fmt.Errorf("error when cleaning up path: %w", err)
	}

	// --------- Option 2 : Drive path
	if isDrivePath {
		log.Tracef("Custom path likely refers to a drive, checking drives, path: '%s', cleanedPath: '%s' ", path, cleanedPath)

		availDisks := map[string]map[string]string{}
		err = l.setDrives(availDisks)
		if err != nil {
			// if setDisks had a problem (e.g. bitlocker locked drive) and did not return
			// the required drive, then pass any possible error on to the caller.
			return err
		}

		for key := range availDisks {
			if !strings.EqualFold(key, cleanedPath) {
				continue
			}

			requiredDrives[path] = utils.CloneStringMap(availDisks[key])
			requiredDrives[path]["id"] = key
			requiredDrives[path]["drive"] = cleanedPath
			requiredDrives[path]["drive_or_id"] = cleanedPath
			requiredDrives[path]["drive_or_name"] = cleanedPath
			requiredDrives[path]["drive_or_name_or_id"] = cleanedPath
			requiredDrives[path]["perflabel_prefix"], _ = getPerflabelPrefix(path)

			return nil
		}
	}

	// --------- Option 3: Path under a volume
	// at this point, the path is checked if its a drive or a network path.
	// if its neither of those, try to open the file/directory
	// check for volumes that include the path in their drive

	_, err = os.Stat(cleanedPath)
	if err != nil && os.IsNotExist(err) {
		log.Debugf("%s: %s", cleanedPath, err.Error())

		return &PartitionNotFoundError{Path: cleanedPath, err: err}
	}

	// try to find closest matching volume
	availVolumes := map[string]map[string]string{}
	l.setVolumes(availVolumes)

	testPath := strings.TrimSuffix(cleanedPath, "\\") + "\\"
	// make first character uppercase because drives are uppercase in the volume list
	if len(testPath) > 1 {
		testPath = strings.ToUpper(testPath[0:1]) + testPath[1:]
	}

	var match *map[string]string

	for i := range availVolumes {
		volume := availVolumes[i]

		// parent fallback means parent folders of a drive are valid as well
		if parentFallback && volume["drive"] != "" &&
			strings.HasPrefix(strings.ToUpper(testPath), strings.ToUpper(volume["drive"])) {
			if match == nil || len((*match)["drive"]) < len(volume["drive"]) {
				match = &volume
			}
		}

		if strings.EqualFold(testPath, volume["drive"]) {
			match = &volume

			break
		}
	}
	if match != nil {
		requiredDrives[path] = utils.CloneStringMap(*match)
		requiredDrives[path]["drive"] = path
		requiredDrives[path]["name"] = path
		requiredDrives[path]["drive_or_name"] = path
		requiredDrives[path]["drive_or_name_or_id"] = path

		return nil
	}

	// add anyway to generate an error later with more default values filled in
	entry := l.driveEntry(path)
	entry["_error"] = fmt.Sprintf("%s not mounted", path)
	requiredDrives[path] = entry

	return nil
}

// adds all network shares to requiredDrives
func (l *CheckDrivesize) setShares(requiredDrives map[string]map[string]string) {
	partitions, err := disk.Partitions(true)
	if err != nil {
		log.Debugf("Error when discovering partitions: %s", err.Error())
	}

	for _, partition := range partitions {
		// skip empty partitions
		if partition.Device == "" && partition.Mountpoint == "" && partition.Fstype == "" {
			continue
		}
		logicalDrive := strings.TrimSuffix(partition.Device, "\\") + "\\"
		driveType, err := GetDriveType(logicalDrive)
		if err != nil {
			log.Debugf("Error when getting the drive type for logical drive %s network drives: %s", logicalDrive, err.Error())

			continue
		}

		if driveType == DriveRemote {
			remoteName, err := NetGetConnection(logicalDrive[0 : len(logicalDrive)-1])
			if err != nil {
				log.Debugf("Error when getting the connection for logical drive %s: %s", logicalDrive, err.Error())

				continue
			}
			log.Debugf("Logical Drive: %s, Drive Type: %d, Remote name: %s", logicalDrive, driveType, remoteName)
			// modify existing drive if its there
			// if its not, add it new
			drive, ok := requiredDrives[logicalDrive]
			if !ok {
				drive = make(map[string]string)
			}
			drive["id"] = remoteName
			drive["drive"] = logicalDrive
			drive["drive_or_id"] = logicalDrive
			drive["drive_or_name"] = logicalDrive
			drive["drive_or_name_or_id"] = logicalDrive

			drive["letter"] = fmt.Sprintf("%c", logicalDrive[0])
			drive["remote_name"] = remoteName
			if isNetworkDrivePersistent(logicalDrive) {
				drive["persistent"] = "1"
			} else {
				drive["persistent"] = "0"
			}
			requiredDrives[logicalDrive] = drive
		}
	}
}

// driveLetter is assumed to be like 'X:\'
func isNetworkDrivePersistent(driveLetter string) (isPersistent bool) {
	driveLetter = driveLetter[0:1]
	persistentNetworkDrives, err := discoverPersistentNetworkDrives()
	if err != nil {
		log.Debugf("Error when discovering persistent network drives: %s", err.Error())
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
