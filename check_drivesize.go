package snclient

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/shirou/gopsutil/v3/disk"
	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_drivesize"] = CheckEntry{"check_drivesize", new(CheckDrivesize)}
}

type CheckDrivesize struct {
	noCopy noCopy
	data   CheckData
}

/* check_drivesize
 * Description: Checks the drive usage on the host.
 * Thresholds: used, free, used_pct, free_pct
 * Units: B, KB, MB, GB, TB, %
 */
func (l *CheckDrivesize) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := CheckExitOK
	l.data.detailSyntax = "%(drive_or_name)\\: %(used)/%(size) used"
	l.data.okSyntax = "All %(count) drive(s) are ok"
	l.data.topSyntax = "%(problem_list)"
	argList, err := ParseArgs(args, &l.data)
	if err != nil {
		return nil, fmt.Errorf("args error: %s", err.Error())
	}
	var output string
	drives := []string{}
	excludes := []string{}
	var checkData map[string]string

	// parse threshold args
	for _, arg := range argList {
		switch arg.key {
		case "drive":
			drives = append(drives, strings.Split(strings.ToUpper(arg.value), ",")...)
		case "exclude":
			excludes = append(excludes, strings.Split(strings.ToUpper(arg.value), ",")...)
		}
	}

	perfMetrics := []*CheckMetric{}

	// collect disk metrics
	disks, _ := disk.Partitions(true)

	disksOk := make([]string, 0, len(disks))
	disksWarning := make([]string, 0, len(disks))
	disksCritical := make([]string, 0, len(disks))

	for _, drive := range disks {
		if len(drives) > 0 && !slices.Contains(drives, "*") &&
			!slices.Contains(drives, drive.Mountpoint) && !slices.Contains(drives, "all-drives") {
			continue
		}

		if slices.Contains(excludes, drive.Mountpoint) {
			continue
		}

		usage, _ := disk.Usage(drive.Mountpoint)

		metrics := map[string]string{
			"drive_or_name":  drive.Mountpoint,
			"total_used_pct": strconv.FormatUint(uint64(usage.UsedPercent), 10),
			"total free_pct": strconv.FormatUint(usage.Free*100/(usage.Total+1), 10),
			"used":           humanize.Bytes(usage.Used),
			"free":           humanize.Bytes(usage.Free),
			"size":           humanize.Bytes(usage.Total),
			"used_pct":       strconv.FormatUint(uint64(usage.UsedPercent), 10),
			"free_pct":       strconv.FormatUint(usage.Free*100/(usage.Total+1), 10),
		}

		for key, val := range metrics {
			var value float64
			unit := ""
			if l.data.warnThreshold.unit == "%" {
				value, _ = strconv.ParseFloat(val, 64)
			} else {
				f, _ := strconv.ParseFloat(val, 64)
				value, unit = humanize.ComputeSI(f)
			}
			perfMetrics = append(perfMetrics, &CheckMetric{
				Name:  fmt.Sprintf("%v %v", drive.Mountpoint, key),
				Unit:  unit,
				Value: math.Round(value * 1e3 / 1e3),
			})
		}

		if CompareMetrics(metrics, l.data.critThreshold) {
			disksCritical = append(disksCritical, ParseSyntax(l.data.detailSyntax, metrics))

			continue
		}

		if CompareMetrics(metrics, l.data.warnThreshold) {
			disksWarning = append(disksWarning, ParseSyntax(l.data.detailSyntax, metrics))

			continue
		}

		disksOk = append(disksOk, ParseSyntax(l.data.detailSyntax, metrics))
	}

	if len(disksCritical) > 0 {
		state = CheckExitCritical
	} else if len(disksWarning) > 0 {
		state = CheckExitWarning
	}

	checkData = map[string]string{
		"status":       strconv.FormatInt(state, 10),
		"count":        strconv.FormatInt(int64(len(drives)), 10),
		"ok_list":      strings.Join(disksOk, ", "),
		"warn_list":    strings.Join(disksWarning, ", "),
		"crit_list":    strings.Join(disksCritical, ", "),
		"problem_list": strings.Join(append(disksCritical, disksWarning...), ", "),
	}

	if state == CheckExitOK {
		output = ParseSyntax(l.data.okSyntax, checkData)
	} else {
		output = ParseSyntax(l.data.topSyntax, checkData)
	}

	return &CheckResult{
		State:   state,
		Output:  output,
		Metrics: perfMetrics,
	}, nil
}
