package snclient

import (
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
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
		okSyntax:     "All %(count) drive(s) are ok",
		detailSyntax: "%(drive_or_name)\\: %(used)/%(size) used",
		topSyntax:    "%(problem_list)",
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
	disks, _ := disk.Partitions(true)

	for _, drive := range disks {
		if len(drives) > 0 && !slices.Contains(drives, "*") &&
			!slices.Contains(drives, drive.Mountpoint) && !slices.Contains(drives, "all-drives") {
			continue
		}

		if slices.Contains(excludes, drive.Mountpoint) {
			continue
		}

		usage, _ := disk.Usage(drive.Mountpoint)

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

		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			Name:  drive.Mountpoint,
			Unit:  "",
			Value: usage.UsedPercent,
		})
	}

	check.Finalize()

	return check.result, nil
}
