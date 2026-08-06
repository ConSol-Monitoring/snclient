package snclient

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/utils"
	"github.com/shirou/gopsutil/v4/disk"
)

func init() {
	AvailableChecks["check_mount"] = CheckEntry{"check_mount", NewCheckMount}
}

type CheckMount struct {
	mountPoints   []string
	expectOptions string
	expectFSType  string
}

func NewCheckMount() CheckHandler {
	return &CheckMount{}
}

func (l *CheckMount) Build() *CheckData {
	return &CheckData{
		name:         "check_mount",
		description:  "Checks the status for a mounted filesystem",
		implemented:  ALL,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"mount":   {value: &l.mountPoints, description: "The mount point to check"},
			"options": {value: &l.expectOptions, description: "The mount options to expect"},
			"fstype":  {value: &l.expectFSType, description: "The fstype to expect"},
		},
		detailSyntax:    "mount ${mount} ${issues}",
		okSyntax:        "${status} - ${count} mount(s) found",
		topSyntax:       "${status} - ${problem_list}",
		defaultWarning:  "issues != ''",
		defaultCritical: "issues like 'not mounted'",
		emptyState:      3,
		emptySyntax:     "check_mount failed to find anything with this filter.",
		attributes: []CheckAttribute{
			{name: "mount", description: "Path of mounted folder"},
			{name: "options", description: "Mount options"},
			{name: "device", description: "Device of this mount"},
			{name: "fstype", description: "FS type for this mount"},
			{name: "issues", description: "Issues found"},
		},
		exampleDefault: `
    check_mount
    OK - 3 mounts(s) found

	check_mount mount=/ options=rw,relatime fstype=ext4
	OK - 1 mount(s) found

	check_mount mount=X:
	CRITICAL - mount X: not mounted
	`,
		exampleArgs: `'mount=/' 'options=rw,relatime'`,
	}
}

//nolint:funlen // no need to split this up
func (l *CheckMount) Check(ctx context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	if check.output != OutputInventory && len(l.mountPoints) == 0 && l.expectOptions == "" && l.expectFSType == "" {
		return nil, fmt.Errorf("must specify at least one of mount/options/fstype")
	}

	for idx := range l.mountPoints {
		l.mountPoints[idx] = trimTrailingSeparator(l.mountPoints[idx])
	}

	partitionMap := map[string]bool{}
	partitions, err := l.getDrives(ctx, partitionMap)
	if err != nil {
		return nil, err
	}
	volumes, err := l.getVolumes(ctx, check, partitionMap)
	if err != nil {
		return nil, err
	}
	partitions = append(partitions, volumes...)

	for i, entry := range partitions {
		partition := partitions[i]
		if l.expectFSType != "" && !strings.EqualFold(l.expectFSType, partition["fstype"]) {
			continue
		}
		issues := []string{}
		if l.expectOptions != "" {
			optsWant := strings.Split(l.expectOptions, ",")
			optsWantH := make(map[string]bool)
			for _, opt := range optsWant {
				optsWantH[opt] = true
			}
			optsHaveH := make(map[string]bool)
			for opt := range strings.SplitSeq(partition["options"], ",") {
				optsHaveH[opt] = true
			}
			missing := []string{}
			for k := range optsWantH {
				if _, ok := optsHaveH[k]; !ok {
					missing = append(missing, k)
				}
			}
			if len(missing) > 0 {
				issues = append(issues, fmt.Sprintf("missing options: %s", strings.Join(missing, ", ")))
			}
			exceeding := []string{}
			for k := range optsHaveH {
				if _, ok := optsWantH[k]; !ok {
					exceeding = append(exceeding, k)
				}
			}
			if len(exceeding) > 0 {
				issues = append(issues, fmt.Sprintf("exceeding options: %s", strings.Join(exceeding, ", ")))
			}
		}
		if l.expectFSType != "" && !strings.EqualFold(l.expectFSType, partition["fstype"]) {
			issues = append(issues, fmt.Sprintf("expected fstype differs: %s != %s", l.expectFSType, partition["fstype"]))
		}
		if len(issues) > 0 {
			entry["issues"] = strings.Join(issues, ", ")
		}
		check.listData = append(check.listData, entry)
	}

	// check if a mountpoint was supplied but not yet found
	if len(l.mountPoints) > 0 {
		for _, mountPoint := range l.mountPoints {
			if _, ok := partitionMap[mountPoint]; !ok {
				entry := map[string]string{
					"mount":   mountPoint,
					"device":  "",
					"fstype":  "",
					"options": "",
					"issues":  "not mounted",
				}
				check.listData = append(check.listData, entry)
			}
		}
	}

	return check.Finalize()
}

func (l *CheckMount) getDrives(ctx context.Context, partitionMap map[string]bool) (drives []map[string]string, err error) {
	excludes := defaultExcludedFsTypes()
	excludes = append(excludes, "tmpfs")
	partitions, err := disk.PartitionsWithContext(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get mounts: %s", err.Error())
	}

	for i := range partitions {
		partition := partitions[i]
		mountpoint := trimTrailingSeparator(partition.Mountpoint)
		partitionMap[mountpoint] = true
		if len(l.mountPoints) > 0 {
			if !slices.Contains(l.mountPoints, mountpoint) {
				log.Tracef("skipped mountpoint: %s - not matching mount argument", mountpoint)

				continue
			}
		} else {
			// skip internal filesystems
			if slices.Contains(excludes, partition.Fstype) {
				log.Tracef("skipped mountpoint: %s - fstype %s is excluded", mountpoint, partition.Fstype)

				continue
			}
			// skip some know internal locations
			switch {
			case strings.HasPrefix(mountpoint, "/run"),
				strings.HasPrefix(mountpoint, "/proc"),
				strings.HasPrefix(mountpoint, "/sys"),
				strings.HasPrefix(mountpoint, "/dev"):

				log.Tracef("skipped mountpoint: %s - prefix matched internal system mounts", mountpoint)

				continue
			}
		}

		if partition.Fstype == "" && partition.Device == "" && mountpoint == "" {
			log.Tracef("skipped mountpoint: %s - empty device, fstype and mountpoint", mountpoint)

			continue
		}

		device := utils.ReplaceCommonPasswordPattern(partition.Device)
		entry := map[string]string{
			"mount":   mountpoint,
			"device":  device,
			"fstype":  partition.Fstype,
			"options": strings.Join(partition.Opts, ","),
			"issues":  "",
		}
		drives = append(drives, entry)
	}

	return drives, nil
}

// trimTrailingSeparator removes a trailing path separator, but keeps a single-character mountpoint like "/" or "C:" intact so that it stays a valid root path.
func trimTrailingSeparator(path string) string {
	if len(path) > 1 {
		return strings.TrimSuffix(path, string(os.PathSeparator))
	}

	return path
}
