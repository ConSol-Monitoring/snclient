package snclient

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"pkg/utils"

	"github.com/dustin/go-humanize"
	"github.com/shirou/gopsutil/v3/disk"
	"golang.org/x/sys/windows"
)

func init() {
	AvailableChecks["check_drivesize"] = CheckEntry{"check_drivesize", new(CheckDrivesize)}
}

const (
	IoctlStorageBase           = 0x2D
	FileDeviceMassStorage      = 0x0000002d
	MethodBuffered             = 0x0
	FileAnyAccess              = 0x0
	StorageGetHotplugInfo      = 0x0305
	IoctlStorageGetHotplugInfo = (IoctlStorageBase << 16) | (FileAnyAccess << 14) | (StorageGetHotplugInfo << 2) | MethodBuffered

	// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getvolumeinformationw
	FileReadOonlyVolume = uint32(0x00080000)
)

type CheckDrivesize struct{}

/* check_drivesize
 * Description: Checks the drive usage on this windows host.
 */
func (l *CheckDrivesize) Check(_ *Agent, args []string) (*CheckResult, error) {
	drives := []string{}
	excludes := []string{}
	total := false
	magic := float64(1)
	check := &CheckData{
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]interface{}{
			"drive":   &drives,
			"exclude": &excludes,
			"total":   &total,
			"magic":   &magic,
		},
		defaultFilter:   "( mounted = 1  or media_type = 0 )",
		defaultWarning:  "used_pct > 80",
		defaultCritical: "used_pct > 90",
		okSyntax:        "%(status) All %(count) drive(s) are ok",
		detailSyntax:    "%(drive_or_name) %(used)/%(size) used",
		topSyntax:       "${status} ${problem_list}",
		emptySyntax:     "%(status): No drives found",
	}
	_, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	check.SetDefaultThresholdUnit("%", []string{"used", "free"})
	check.ExpandThresholdUnit([]string{"k", "m", "g", "p", "e", "ki", "mi", "gi", "pi", "ei"}, "B", []string{"used", "free"})

	requiredDisks, err := l.getRequiredDisks(drives)
	if err != nil {
		return nil, err
	}

	for _, disk := range requiredDisks {
		if l.isExcluded(disk, excludes) {
			continue
		}
		l.addDiskDetails(check, disk, magic)
		disk["flags"] = strings.Join(l.getFlagNames(disk), ", ")
		check.listData = append(check.listData, disk)
	}

	if total {
		// totals go first, so save current metrics and add them again
		tmpMetrics := check.result.Metrics
		check.result.Metrics = make([]*CheckMetric, 0)
		l.addTotal(check)
		check.result.Metrics = append(check.result.Metrics, tmpMetrics...)
	}

	return check.Finalize()
}

func (l *CheckDrivesize) isExcluded(drive map[string]string, excludes []string) bool {
	for _, exclude := range excludes {
		if strings.EqualFold(exclude, drive["drive"]) {
			return true
		}
		if strings.EqualFold(exclude+":\\", drive["drive"]) {
			return true
		}
		if strings.EqualFold(exclude+"\\", drive["drive"]) {
			return true
		}
	}

	return false
}

func (l *CheckDrivesize) getRequiredDisks(drives []string) (requiredDisks map[string]map[string]string, err error) {
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
			// "c" or "c:"" will use the drive c and "c:\" will use the volume
			found := false
			switch len(drive) {
			case 1, 2:
				drive = strings.TrimSuffix(drive, ":") + ":"
				availDisks := map[string]map[string]string{}
				err := l.setDisks(availDisks)
				if err != nil {
					return nil, err
				}
				for driveOrID := range availDisks {
					if strings.EqualFold(driveOrID, drive+"\\") {
						requiredDisks[drive] = availDisks[driveOrID]
						requiredDisks[drive]["drive"] = drive
						found = true
					}
				}
			}
			if !found {
				requiredDisks[drive] = map[string]string{
					"id":            "",
					"drive":         drive,
					"drive_or_id":   drive,
					"drive_or_name": drive,
					"letter":        "",
				}
			}
		}
	}

	return requiredDisks, nil
}

func (l *CheckDrivesize) addDiskDetails(check *CheckData, drive map[string]string, magic float64) {
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

	err := l.setDeviceFlags(drive)
	if err != nil {
		drive["_error"] = fmt.Sprintf("device flags: %s", err.Error())

		return
	}

	l.setDeviceInfo(drive)

	usage, err := disk.Usage(drive["drive_or_id"])
	if err != nil {
		if drive["type"] != "cdrom" && drive["removable"] == "1" {
			drive["_error"] = fmt.Sprintf("failed to get size for %s: %s", drive["drive_or_id"], err.Error())
		}
		usage = &disk.UsageStat{}
	}

	freePct := float64(0)
	if usage.Total > 0 {
		freePct = float64(usage.Free) * 100 / (float64(usage.Total))
	}

	drive["mounted"] = "1"
	drive["size"] = humanize.IBytes(uint64(magic * float64(usage.Total)))
	drive["size_bytes"] = fmt.Sprintf("%d", uint64(magic*float64(usage.Total)))
	drive["used"] = humanize.IBytes(uint64(magic * float64(usage.Used)))
	drive["used_bytes"] = fmt.Sprintf("%d", uint64(magic*float64(usage.Used)))
	drive["used_pct"] = fmt.Sprintf("%f", usage.UsedPercent)
	drive["free"] = humanize.IBytes(uint64(magic * float64(usage.Free)))
	drive["free_bytes"] = fmt.Sprintf("%d", uint64(magic*float64(usage.Free)))
	drive["free_pct"] = fmt.Sprintf("%f", freePct)

	if check.HasThreshold("free") {
		l.addMetrics(check, drive["drive"]+" free", magic*float64(usage.Free), magic*float64(usage.Total))
	}
	l.addMetrics(check, drive["drive"]+" used", magic*float64(usage.Used), magic*float64(usage.Total))
}

func (l *CheckDrivesize) addTotal(check *CheckData) {
	total := int64(0)
	free := int64(0)
	used := int64(0)

	for _, disk := range check.listData {
		sizeBytes, err := strconv.ParseInt(disk["size_bytes"], 10, 64)
		if err != nil {
			continue
		}
		freeBytes, err := strconv.ParseInt(disk["free_bytes"], 10, 64)
		if err != nil {
			continue
		}
		usedBytes, err := strconv.ParseInt(disk["used_bytes"], 10, 64)
		if err != nil {
			continue
		}
		free += freeBytes
		total += sizeBytes
		used += usedBytes
	}

	if total == 0 {
		return
	}

	usedPct := float64(used) * 100 / (float64(total))

	drive := map[string]string{
		"id":            "total",
		"name":          "total",
		"drive_or_id":   "total",
		"drive_or_name": "total",
		"drive":         "total",
		"media_type":    "0",
		"type":          "total",
		"letter":        "",
		"size":          humanize.IBytes(uint64(total)),
		"size_bytes":    fmt.Sprintf("%d", total),
		"used":          humanize.IBytes(uint64(used)),
		"used_bytes":    fmt.Sprintf("%d", used),
		"used_pct":      fmt.Sprintf("%f", usedPct),
		"free":          humanize.IBytes(uint64(free)),
		"free_bytes":    fmt.Sprintf("%d", free),
		"free_pct":      fmt.Sprintf("%f", float64(free)*100/(float64(total))),
	}
	check.listData = append(check.listData, drive)

	if check.HasThreshold("free") {
		l.addMetrics(check, drive["drive"]+" free", float64(free), float64(total))
	}
	l.addMetrics(check, drive["drive"]+" used", float64(used), float64(total))
}

func (l *CheckDrivesize) setDeviceFlags(drive map[string]string) error {
	szDevice := fmt.Sprintf(`\\.\%s`, strings.TrimSuffix(drive["drive"], "\\"))
	szPtr, err := syscall.UTF16PtrFromString(szDevice)
	if err != nil {
		log.Warnf("stringPtr: %s", szDevice, err.Error())

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
	err = windows.DeviceIoControl(handle, IoctlStorageGetHotplugInfo, nil, 0, (*byte)(unsafe.Pointer(&hotplugInfo)), uint32(unsafe.Sizeof(hotplugInfo)), &num, nil)
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

func (l *CheckDrivesize) setDeviceInfo(drive map[string]string) {
	volPtr, err := syscall.UTF16PtrFromString(drive["drive_or_id"])
	if err != nil {
		log.Warnf("stringPtr: %s", drive["drive_or_id"], err.Error())

		return
	}
	drive["type"] = l.driveType(windows.GetDriveType(volPtr))

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
		if drive["type"] != "cdrom" && drive["removable"] == "1" {
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
	drive["fstype"] = syscall.UTF16ToString(fileSystemName)
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
	if err != nil {
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
		requiredDisks[drive] = entry
	}

	return
}

func (l *CheckDrivesize) setVolumes(requiredDisks map[string]map[string]string) {
	volumes := []string{}
	volumeName := make([]uint16, 512)
	hndl, err := windows.FindFirstVolume(&volumeName[0], uint32(len(volumeName)))
	if err != nil {
		log.Tracef("FindFirstVolume: %w: %s", err, err.Error())

		return
	}
	volumes = append(volumes, syscall.UTF16ToString(volumeName))

	for {
		err := windows.FindNextVolume(hndl, &volumeName[0], uint32(len(volumeName)))
		if err != nil {
			log.Tracef("FindNextVolume: %w: %s", err, err.Error())

			break
		}
		volumes = append(volumes, syscall.UTF16ToString(volumeName))
	}

	for _, vol := range volumes {
		volPtr, err := syscall.UTF16PtrFromString(vol)
		if err != nil {
			log.Warnf("stringPtr: %s", vol, err.Error())

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
		if len(names) > 0 {
			entry["letter"] = fmt.Sprintf("%c", names[0])
		}
		requiredDisks[driveOrID] = entry
	}
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

func (l *CheckDrivesize) addMetrics(check *CheckData, name string, val, total float64) {
	percent := float64(0)
	if strings.Contains(name, "used") {
		percent = 100
	}
	if total > 0 {
		percent = val * 100 / total
	}
	pctName := name + " %"
	check.result.Metrics = append(check.result.Metrics,
		&CheckMetric{
			Name:     name,
			Unit:     "B",
			Value:    val,
			Warning:  check.TransformThreshold(check.warnThreshold, "used", name, "%", "B", total),
			Critical: check.TransformThreshold(check.critThreshold, "used", name, "%", "B", total),
			Min:      &Zero,
			Max:      &total,
		},
		&CheckMetric{
			Name:     pctName,
			Unit:     "%",
			Value:    utils.ToPrecision(percent, 1),
			Warning:  check.TransformThreshold(check.warnThreshold, "used", pctName, "B", "%", total),
			Critical: check.TransformThreshold(check.critThreshold, "used", pctName, "B", "%", total),
			Min:      &Zero,
			Max:      &Hundred,
		},
	)
}
