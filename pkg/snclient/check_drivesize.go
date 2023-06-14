//go:build !windows

package snclient

import (
	"fmt"
	"strconv"
	"strings"

	"pkg/humanize"

	"github.com/shirou/gopsutil/v3/disk"
	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_drivesize"] = CheckEntry{"check_drivesize", new(CheckDrivesize)}
}

type CheckDrivesize struct{}

/* check_drivesize
 * Description: Checks the drive usage on the host.
 * Thresholds: used, free, used_pct, free_pct
 * Units: B, KB, MB, GB, TB, %
 */
func (l *CheckDrivesize) Check(_ *Agent, args []string) (*CheckResult, error) {
	check := &CheckData{
		result: &CheckResult{
			State: CheckExitOK,
		},
		defaultWarning:  "used_pct > 80",
		defaultCritical: "used_pct > 90",
		okSyntax:        "%(status): All %(count) drive(s) are ok",
		detailSyntax:    "%(drive_or_name) %(used)/%(size) used",
		topSyntax:       "${status}: ${problem_list}",
		emptySyntax:     "%(status): No drives found",
	}
	argList, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	drives := []string{}
	excludes := []string{}

	// parse threshold args
	for _, arg := range argList {
		switch arg.key {
		case "drive":
			drives = append(drives, strings.Split(strings.ToUpper(arg.value), ",")...)
		case "exclude":
			excludes = append(excludes, strings.Split(strings.ToUpper(arg.value), ",")...)
		}
	}

	// collect disk metrics
	disks, err := disk.Partitions(true)
	if err != nil {
		return nil, fmt.Errorf("disk partitions failed: %s", err.Error())
	}

	for _, drive := range disks {
		if len(drives) > 0 && !slices.Contains(drives, "*") &&
			!slices.Contains(drives, drive.Mountpoint) && !slices.Contains(drives, "all-drives") {
			continue
		}

		if slices.Contains(excludes, drive.Mountpoint) {
			continue
		}

		usage, err := disk.Usage(drive.Mountpoint)
		if err != nil {
			log.Debugf("disk usage %s failed: %s", drive.Mountpoint, err.Error())

			continue
		}

		check.listData = append(check.listData, map[string]string{
			"drive_or_name":  drive.Mountpoint,
			"total_used_pct": strconv.FormatUint(uint64(usage.UsedPercent), 10),
			"total free_pct": strconv.FormatUint(usage.Free*100/(usage.Total+1), 10),
			"used":           humanize.Bytes(usage.Used),
			"free":           humanize.Bytes(usage.Free),
			"size":           humanize.Bytes(usage.Total),
			"used_pct":       strconv.FormatUint(uint64(usage.UsedPercent), 10),
			"free_pct":       strconv.FormatUint(usage.Free*100/(usage.Total+1), 10),
		})

		check.result.Metrics = append(check.result.Metrics,
			&CheckMetric{
				Name:     drive.Mountpoint + " used %",
				Unit:     "%",
				Value:    usage.UsedPercent,
				Warning:  check.warnThreshold,
				Critical: check.critThreshold,
			},
			&CheckMetric{
				Name:  drive.Mountpoint + " used",
				Unit:  "B",
				Value: float64(usage.Used),
			},
		)
	}

	return check.Finalize()
}
