//go:build !windows

package snclient

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"pkg/humanize"
	"pkg/utils"

	"github.com/shirou/gopsutil/v3/disk"
)

func init() {
	AvailableChecks["check_drivesize"] = CheckEntry{"check_drivesize", new(CheckDrivesize)}
}

type CheckDrivesize struct {
	drives           []string
	excludes         []string
	total            bool
	magic            float64
	mounted          bool
	ignoreUnreadable bool
	hasCustomPath    bool
}

func (l *CheckDrivesize) defaultExcludedFsTypes() []string {
	return []string{
		"binfmt_misc",
		"bpf",
		"cgroup2fs",
		"configfs",
		"debugfs",
		"devpts",
		"efivarfs",
		"fusectl",
		"hugetlbfs",
		"mqueue",
		"nfsd",
		"proc",
		"pstorefs",
		"ramfs",
		"rpc_pipefs",
		"securityfs",
		"sysfs",
		"tracefs",
	}
}

func (l *CheckDrivesize) Build() *CheckData {
	l.drives = []string{"all"}
	l.excludes = []string{}
	l.total = false
	l.magic = float64(1)
	l.mounted = false
	l.ignoreUnreadable = false
	l.hasCustomPath = false

	return &CheckData{
		name:         "check_drivesize",
		description:  "Checks the disk drive/volumes usage on a host.",
		implemented:  ALL,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"drive":   {value: &l.drives, isFilter: true, description: "The drives to check"},
			"exclude": {value: &l.excludes, description: "List of drives to exclude from check"},
			"total":   {value: &l.total, description: "Include the total of all matching drives"},
			"magic": {value: &l.magic, description: "Magic number for use with scaling drive sizes. " +
				"Note there is also a more generic magic factor in the perf-config option."},
			"mounted":           {value: &l.mounted, description: "Deprecated, use filter instead"},          // deprecated and unused, but should not result in unknown argument
			"ignore-unreadable": {value: &l.ignoreUnreadable, description: "Deprecated, use filter instead"}, // same
		},
		defaultFilter:   "fstype not in (" + utils.List2String(l.defaultExcludedFsTypes()) + ")",
		defaultWarning:  "used_pct > 80",
		defaultCritical: "used_pct > 90",
		okSyntax:        "%(status): All %(count) drive(s) are ok",
		detailSyntax:    "%(drive_or_name) %(used)/%(size) used",
		topSyntax:       "${status}: ${problem_list}",
		emptyState:      CheckExitUnknown,
		emptySyntax:     "%(status): No drives found",
		attributes: []CheckAttribute{
			{name: "drive", description: "Technical name of drive"},
			{name: "name", description: "Descriptive name of drive"},
			{name: "id", description: "Drive or id of drive"},
			{name: "drive_or_id", description: "Drive letter if present if not use id"},
			{name: "drive_or_name", description: "Drive letter if present if not use name"},
			{name: "fstype", description: "Filesystem type"},
			{name: "free", description: "Free (human readable) bytes"},
			{name: "free_bytes", description: "Number of free bytes"},
			{name: "free_pct", description: "Free bytes in percent"},
			{name: "inodes_free", description: "Number of free inodes"},
			{name: "inodes_free_pct", description: "Number of free inodes in percent"},
			{name: "inodes_total", description: "Number of total free inodes"},
			{name: "inodes_used", description: "Number of used inodes"},
			{name: "inodes_used_pct", description: "Number of used inodes in percent"},
			{name: "mounted", description: "Flag wether drive is mounter (0/1)"},
			{name: "size", description: "Total size in human readable bytes"},
			{name: "size_bytes", description: "Total size in bytes"},
			{name: "used", description: "Used (human readable) bytes"},
			{name: "used_bytes", description: "Number of used bytes"},
			{name: "used_pct", description: "Used bytes in percent"},
		},
		exampleDefault: `
    check_drivesize drive=/
    OK: All 1 drive(s) are ok |'/ used'=296820846592B;;;0;489570443264 '/ used %'=60.6%;;;0;100
	`,
		exampleArgs: `'warn=used_pct > 90' 'crit=used_pct > 95'`,
	}
}

func (l *CheckDrivesize) Check(_ context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	enabled, _, _ := snc.Config.Section("/modules").GetBool("CheckDisk")
	if !enabled {
		return nil, fmt.Errorf("module CheckDisk is not enabled in /modules section")
	}

	check.SetDefaultThresholdUnit("%", []string{"used", "free"})
	check.ExpandThresholdUnit([]string{"k", "m", "g", "p", "e", "ki", "mi", "gi", "pi", "ei"}, "B", []string{"used", "free"})

	requiredDisks, err := l.getRequiredDisks(l.drives)
	if err != nil {
		return nil, err
	}

	// sort by drive / id
	keys := make([]string, 0, len(requiredDisks))
	for k := range requiredDisks {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		drive := requiredDisks[k]
		if l.isExcluded(drive, l.excludes) {
			continue
		}
		l.addDiskDetails(check, drive, l.magic)
		check.listData = append(check.listData, drive)
	}

	if l.total {
		// totals go first, so save current metrics and add them again
		tmpMetrics := check.result.Metrics
		check.result.Metrics = make([]*CheckMetric, 0)
		l.addTotal(check)
		check.result.Metrics = append(check.result.Metrics, tmpMetrics...)
	}

	// remove errored paths unless custom path is specified
	if !l.hasCustomPath {
		for i, entry := range check.listData {
			if errMsg, ok := entry["_error"]; ok {
				log.Debugf("drivesize failed for %s: %s", entry["drive_or_id"], errMsg)
				check.listData[i]["_skip"] = "1"
			}
		}
	}

	return check.Finalize()
}

func (l *CheckDrivesize) isExcluded(drive map[string]string, excludes []string) bool {
	for _, exclude := range excludes {
		if strings.EqualFold(exclude, drive["drive"]) {
			return true
		}
		if strings.EqualFold(exclude+"/", drive["drive"]) {
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
		case "*", "all", "all-drives":
			err := l.setDisks(requiredDisks)
			if err != nil {
				return nil, err
			}
		case "all-volumes":
			// nothing appropriate on linux
		default:
			l.hasCustomPath = true
			err := l.setCustomPath(drive, requiredDisks)
			if err != nil {
				return nil, err
			}
		}
	}

	return requiredDisks, nil
}

func (l *CheckDrivesize) setDisks(requiredDisks map[string]map[string]string) (err error) {
	partitions, err := disk.Partitions(true)
	if err != nil {
		return fmt.Errorf("disk partitions failed: %s", err.Error())
	}
	for _, partition := range partitions {
		drive := partition.Mountpoint
		entry, ok := requiredDisks[drive]
		if !ok {
			entry = make(map[string]string)
		}

		entry["drive"] = drive
		entry["drive_or_id"] = drive
		entry["drive_or_name"] = drive
		requiredDisks[drive] = entry
	}

	return
}

func (l *CheckDrivesize) setCustomPath(drive string, requiredDisks map[string]map[string]string) (err error) {
	// try to find closes matching mount
	availMounts := map[string]map[string]string{}
	err = l.setDisks(availMounts)
	if err != nil {
		return err
	}

	var match *map[string]string
	for i := range availMounts {
		vol := availMounts[i]
		if vol["drive"] != "" && strings.HasPrefix(drive, vol["drive"]) {
			if match == nil || len((*match)["drive"]) < len(vol["drive"]) {
				match = &vol
			}
		}
		// direct match, no need to search further
		if drive == vol["drive"] {
			break
		}
	}
	if match != nil {
		requiredDisks[drive] = utils.CloneStringMap(*match)
		requiredDisks[drive]["drive"] = drive

		return nil
	}

	// add anyway to generate an error later with more default values filled in
	requiredDisks[drive] = map[string]string{
		"id":            "",
		"drive":         drive,
		"drive_or_id":   drive,
		"drive_or_name": drive,
	}

	return nil
}

func (l *CheckDrivesize) addDiskDetails(check *CheckData, drive map[string]string, magic float64) {
	// set some defaults
	drive["id"] = ""
	drive["name"] = ""
	drive["mounted"] = "0"

	usage, err := disk.Usage(drive["drive_or_id"])
	if err != nil {
		drive["_error"] = fmt.Sprintf("Failed to find disk partition %s: %s", drive["drive_or_id"], err.Error())
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
	drive["inodes_total"] = fmt.Sprintf("%d", usage.InodesTotal)
	drive["inodes_used"] = fmt.Sprintf("%d", usage.InodesUsed)
	drive["inodes_free"] = fmt.Sprintf("%d", usage.InodesFree)
	drive["inodes_used_pct"] = fmt.Sprintf("%f", usage.InodesUsedPercent)
	drive["inodes_free_pct"] = fmt.Sprintf("%f", 100-usage.InodesUsedPercent)
	drive["fstype"] = usage.Fstype

	// check filter before adding metrics
	if !check.MatchMapCondition(check.filter, drive) {
		return
	}

	if check.HasThreshold("free") || check.HasThreshold("free_pct") {
		check.AddBytePercentMetrics("free", drive["drive"]+" free", magic*float64(usage.Free), magic*float64(usage.Total))
	}
	if check.HasThreshold("used") || check.HasThreshold("used_pct") {
		check.AddBytePercentMetrics("used", drive["drive"]+" used", magic*float64(usage.Used), magic*float64(usage.Total))
	}
	if check.HasThreshold("inodes_used_pct") {
		check.AddPercentMetrics("inodes_used_pct", drive["drive"]+" inodes", float64(usage.InodesUsed), float64(usage.InodesTotal))
	}
	if check.HasThreshold("inodes_free_pct") {
		check.AddPercentMetrics("inodes_free_pct", drive["drive"]+" free", float64(usage.InodesUsed), float64(usage.InodesTotal))
	}
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
		"size":          humanize.IBytesF(uint64(total), 3),
		"size_bytes":    fmt.Sprintf("%d", total),
		"used":          humanize.IBytesF(uint64(used), 3),
		"used_bytes":    fmt.Sprintf("%d", used),
		"used_pct":      fmt.Sprintf("%f", usedPct),
		"free":          humanize.IBytesF(uint64(free), 3),
		"free_bytes":    fmt.Sprintf("%d", free),
		"free_pct":      fmt.Sprintf("%f", float64(free)*100/(float64(total))),
		"fstype":        "total",
	}
	check.listData = append(check.listData, drive)

	// check filter before adding metrics
	if !check.MatchMapCondition(check.filter, drive) {
		return
	}

	if check.HasThreshold("free") {
		check.AddBytePercentMetrics("free", drive["drive"]+" free", float64(free), float64(total))
	}
	if check.HasThreshold("used") {
		check.AddBytePercentMetrics("used", drive["drive"]+" used", float64(used), float64(total))
	}
}
