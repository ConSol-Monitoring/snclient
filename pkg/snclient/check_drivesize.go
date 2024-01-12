package snclient

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"pkg/humanize"

	"github.com/shirou/gopsutil/v3/disk"
)

func init() {
	AvailableChecks["check_drivesize"] = CheckEntry{"check_drivesize", NewCheckDrivesize}
}

const (
	DiskDetailsTimeout = 30 * time.Second
)

func defaultExcludedFsTypes() []string {
	// grep ^nodev /proc/filesystems | awk '{ print $2 }' | grep -v '^\(nfs\|cifs\|smb\|fuse$\|tmpfs\)' | sort
	return []string{
		"autofs",
		"bdev",
		"binfmt_misc",
		"bpf",
		"cgroup",
		"cgroup2",
		"configfs",
		"cpuset",
		"debugfs",
		"devpts",
		"devtmpfs",
		"efivarfs",
		"fuse.portal",
		"fusectl",
		"hugetlbfs",
		"mqueue",
		"nsfs",
		"overlay",
		"pipefs",
		"proc",
		"pstore",
		"ramfs",
		"rpc_pipefs",
		"securityfs",
		"sockfs",
		"sysfs",
		"tracefs",
	}
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

func NewCheckDrivesize() CheckHandler {
	return &CheckDrivesize{
		magic:  1,
		drives: []string{"all"},
	}
}

func (l *CheckDrivesize) Build() *CheckData {
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
		defaultFilter:   l.getDefaultFilter(),
		defaultWarning:  "used_pct > 80",
		defaultCritical: "used_pct > 90",
		okSyntax:        "%(status) - All %(count) drive(s) are ok",
		detailSyntax:    "%(drive_or_name) %(used)/%(size) (%(used_pct | fmt=%.1f )%)",
		topSyntax:       "%(status) - ${problem_list}",
		emptyState:      CheckExitUnknown,
		emptySyntax:     "%(status) - No drives found",
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

			{name: "media_type", description: "windows only: numeric media type of drive"},
			{name: "type", description: "windows only: type of drive, ex.: fixed, cdrom, ramdisk,..."},
			{name: "readable", description: "windows only: flag drive is readable (0/1)"},
			{name: "writable", description: "windows only: flag drive is writable (0/1)"},
			{name: "removable", description: "windows only: flag drive is removable (0/1)"},
			{name: "erasable", description: "windows only: flag wether if drive is erasable (0/1)"},
			{name: "hotplug", description: "windows only: flag drive is hotplugable (0/1)"},
		},
		exampleDefault: l.getExample(),
		exampleArgs:    `'warn=used_pct > 90' 'crit=used_pct > 95'`,
	}
}

func (l *CheckDrivesize) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	enabled, _, _ := snc.Config.Section("/modules").GetBool("CheckDisk")
	if !enabled {
		return nil, fmt.Errorf("module CheckDisk is not enabled in /modules section")
	}

	check.SetDefaultThresholdUnit("%", []string{"used_pct", "used", "free", "free_pct", "inodes", "inodes_free"})
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
		l.addDiskDetails(ctx, check, drive, l.magic)
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

	// make sure fstype exists and is lowercase
	for i := range check.listData {
		check.listData[i]["fstype"] = strings.ToLower(check.listData[i]["fstype"])
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

func (l *CheckDrivesize) addMetrics(drive string, check *CheckData, usage *disk.UsageStat, magic float64) {
	total := usage.Used + usage.Free // use this total instead of usage.Total to account in the root reserved space

	if check.HasThreshold("free") || check.HasThreshold("free_pct") {
		check.warnThreshold = check.TransformMultipleKeywords([]string{"free_pct"}, "free", check.warnThreshold)
		check.critThreshold = check.TransformMultipleKeywords([]string{"free_pct"}, "free", check.critThreshold)
		check.AddBytePercentMetrics("free", drive+" free", magic*float64(usage.Free), magic*float64(total))
	}
	if check.HasThreshold("used") || check.HasThreshold("used_pct") {
		check.warnThreshold = check.TransformMultipleKeywords([]string{"used_pct"}, "used", check.warnThreshold)
		check.critThreshold = check.TransformMultipleKeywords([]string{"used_pct"}, "used", check.critThreshold)
		check.AddBytePercentMetrics("used", drive+" used", magic*float64(usage.Used), magic*float64(total))
	}
	if check.HasThreshold("inodes") || check.HasThreshold("inodes_used") || check.HasThreshold("inodes_used_pct") {
		check.warnThreshold = check.TransformMultipleKeywords([]string{"inodes_used_pct", "inodes_used"}, "inodes", check.warnThreshold)
		check.critThreshold = check.TransformMultipleKeywords([]string{"inodes_used_pct", "inodes_used"}, "inodes", check.critThreshold)
		check.AddPercentMetrics("inodes", drive+" inodes", float64(usage.InodesUsed), float64(usage.InodesTotal))
	}
	if check.HasThreshold("inodes_free") || check.HasThreshold("inodes_free_pct") {
		check.warnThreshold = check.TransformMultipleKeywords([]string{"inodes_free_pct"}, "inodes_free", check.warnThreshold)
		check.critThreshold = check.TransformMultipleKeywords([]string{"inodes_free_pct"}, "inodes_free", check.critThreshold)
		check.AddPercentMetrics("inodes_free", drive+" inodes free", float64(usage.InodesFree), float64(usage.InodesTotal))
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
	if !check.MatchMapCondition(check.filter, drive, true) {
		return
	}

	if check.HasThreshold("free") {
		check.AddBytePercentMetrics("free", drive["drive"]+" free", float64(free), float64(total))
	}
	if check.HasThreshold("used") {
		check.AddBytePercentMetrics("used", drive["drive"]+" used", float64(used), float64(total))
	}
}
