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
	mountPoint    string
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
			"mount":   {value: &l.mountPoint, description: "The mount point to check"},
			"options": {value: &l.expectOptions, description: "The mount options to expect"},
			"fstype":  {value: &l.expectFSType, description: "The fstype to expect"},
		},
		detailSyntax:    "mount ${mount} ${issues}",
		okSyntax:        "${status} - mounts are as expected",
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
    check_mount mount=/ options=rw,relatime fstype=ext4
    OK - mounts are as expected
	`,
		exampleArgs: `'mount=/' 'options=rw,relatime'`,
	}
}

func (l *CheckMount) Check(ctx context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	if len(l.mountPoint) > 1 {
		l.mountPoint = strings.TrimSuffix(l.mountPoint, string(os.PathSeparator))
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
	if l.mountPoint != "" {
		if _, ok := partitionMap[l.mountPoint]; !ok {
			entry := map[string]string{
				"mount":   l.mountPoint,
				"device":  "",
				"fstype":  "",
				"options": "",
				"issues":  "not mounted",
			}
			check.listData = append(check.listData, entry)
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
		partitionMap[partition.Mountpoint] = true
		if l.mountPoint != "" {
			if partition.Mountpoint != l.mountPoint {
				log.Tracef("skipped mountpoint: %s - not matching mount argument", partition.Mountpoint)

				continue
			}
		} else {
			// skip internal filesystems
			if slices.Contains(excludes, partition.Fstype) {
				log.Tracef("skipped mountpoint: %s - fstype %s is excluded", partition.Mountpoint, partition.Fstype)

				continue
			}
			// skip some know internal locations
			switch {
			case strings.HasPrefix(partition.Mountpoint, "/run"),
				strings.HasPrefix(partition.Mountpoint, "/proc"),
				strings.HasPrefix(partition.Mountpoint, "/sys"),
				strings.HasPrefix(partition.Mountpoint, "/dev"):

				log.Tracef("skipped mountpoint: %s - prefix matched internal system mounts", partition.Mountpoint)

				continue
			}
		}

		if partition.Fstype == "" && partition.Device == "" && partition.Mountpoint == "" {
			log.Tracef("skipped mountpoint: %s - empty device, fstype and mountpoint", partition.Mountpoint)

			continue
		}

		device := utils.ReplaceCommonPasswordPattern(partition.Device)
		entry := map[string]string{
			"mount":   partition.Mountpoint,
			"device":  device,
			"fstype":  partition.Fstype,
			"options": strings.Join(partition.Opts, ","),
			"issues":  "",
		}
		drives = append(drives, entry)
	}

	return drives, nil
}
