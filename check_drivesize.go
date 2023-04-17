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
}

/* check_drivesize
 * Description: Checks the drive usage on the host.
 * Tresholds: used, free, used_pct, free_pct
 * Units: B, KB, MB, GB, TB, %
 */
func (l *CheckDrivesize) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(CheckExitOK)
	argList := ParseArgs(args)
	var output string
	var warnTreshold Treshold
	var critTreshold Treshold
	drives := []string{}
	detailSyntax := "%(drive_or_name)\\: %(used)/%(size) used"
	okSyntax := "All %(count) drive(s) are ok"
	topSyntax := "%(problem_list)"
	var checkData map[string]string

	// parse treshold args
	for _, arg := range argList {
		switch arg.key {
		case "warn", "warning":
			warnTreshold = ParseTreshold(arg.value)
		case "crit", "critical":
			critTreshold = ParseTreshold(arg.value)
		case "drive":
			drives = strings.Split(strings.ToUpper(arg.value), ",")
		case "detail-syntax":
			detailSyntax = arg.value
		case "top-syntax":
			topSyntax = arg.value
		case "ok-syntax":
			okSyntax = arg.value
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

		usage, _ := disk.Usage(drive.Mountpoint)

		metrics := []MetricData{
			{name: "used", value: strconv.FormatUint(usage.Used, 10)},
			{name: "free", value: strconv.FormatUint(usage.Free, 10)},
			{name: "used_pct", value: strconv.FormatUint(uint64(usage.UsedPercent), 10)},
			{name: "free_pct", value: strconv.FormatUint(usage.Free*100/usage.Total, 10)},
		}

		sdata := map[string]string{
			"drive_or_name":  drive.Mountpoint,
			"total_used_pct": strconv.FormatUint(uint64(usage.UsedPercent), 10),
			"total free_pct": strconv.FormatUint(usage.Free*100/usage.Total, 10),
			"used":           humanize.Bytes(usage.Used),
			"free":           humanize.Bytes(usage.Free),
			"size":           humanize.Bytes(usage.Total),
		}

		for _, metric := range metrics {
			if metric.name == warnTreshold.name || metric.name == critTreshold.name {
				var value float64
				unit := ""
				if warnTreshold.unit == "%" {
					value, _ = strconv.ParseFloat(metric.value, 64)
				} else {
					f, _ := strconv.ParseFloat(metric.value, 64)
					value, unit = humanize.ComputeSI(f)
				}
				perfMetrics = append(perfMetrics, &CheckMetric{
					Name:  fmt.Sprintf("%v %v", drive.Mountpoint, metric.name),
					Unit:  unit,
					Value: math.Round(value * 1e3 / 1e3),
				})
			}
		}

		if CompareMetrics(metrics, critTreshold) {
			disksCritical = append(disksCritical, ParseSyntax(detailSyntax, sdata))

			continue
		}

		if CompareMetrics(metrics, warnTreshold) {
			disksWarning = append(disksWarning, ParseSyntax(detailSyntax, sdata))

			continue
		}

		disksOk = append(disksOk, ParseSyntax(detailSyntax, sdata))
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
		output = ParseSyntax(okSyntax, checkData)
	} else {
		output = ParseSyntax(topSyntax, checkData)
	}

	return &CheckResult{
		State:   state,
		Output:  output,
		Metrics: perfMetrics,
	}, nil
}
