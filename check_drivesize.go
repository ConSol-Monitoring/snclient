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

/* check_memory todo
* todo .
 */
func (l *CheckDrivesize) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(CheckExitOK)
	argList := ParseArgs(args)
	var warnTreshold Treshold
	var critTreshold Treshold
	drives := []string{}

	// parse treshold args
	for _, arg := range argList {
		switch arg.key {
		case "warn", "warning":
			warnTreshold = ParseTreshold(arg.value)
		case "crit", "critical":
			critTreshold = ParseTreshold(arg.value)
		case "drive":
			drives = strings.Split(strings.ToUpper(arg.value), ",")
		}
	}

	perfMetrics := []*CheckMetric{}

	// collect disk metrics
	disks, _ := disk.Partitions(true)

	disksOk := make([]string, 0, len(disks))
	disksWarning := make([]string, 0, len(disks))
	disksCritical := make([]string, 0, len(disks))

	for _, drive := range disks {
		if len(drives) > 0 && !slices.Contains(drives, "*") && !slices.Contains(drives, drive.Mountpoint) {
			continue
		}

		usage, _ := disk.Usage(drive.Mountpoint)
		info := fmt.Sprintf("%v\\: %v/%v used", drive.Mountpoint, humanize.Bytes(usage.Used), humanize.Bytes(usage.Total))

		metrics := []MetricData{
			{name: "used", value: strconv.FormatUint(usage.Used, 10)},
			{name: "free", value: strconv.FormatUint(usage.Free, 10)},
			{name: "used_pct", value: strconv.FormatUint(uint64(usage.UsedPercent), 10)},
			{name: "free_pct", value: strconv.FormatUint(usage.Free*100/usage.Total, 10)},
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
			disksCritical = append(disksCritical, info)

			continue
		}

		if CompareMetrics(metrics, warnTreshold) {
			disksWarning = append(disksWarning, info)

			continue
		}

		disksOk = append(disksOk, info)
	}

	output := ""

	if len(disksCritical) > 0 {
		output += fmt.Sprintf("critical(%v)", strings.Join(disksCritical, ", "))
		if len(disksWarning) > 0 {
			output += fmt.Sprintf(", warning(%v)", strings.Join(disksWarning, ", "))
		}
		if len(disksOk) > 0 {
			output += ", "
		}
		state = CheckExitCritical
	} else if len(disksWarning) > 0 {
		output += fmt.Sprintf("warning(%v)", strings.Join(disksWarning, ", "))
		if len(disksOk) > 0 {
			output += ", "
		}
		state = CheckExitWarning
	}

	output += strings.Join(disksOk, ", ")

	return &CheckResult{
		State:   state,
		Output:  output,
		Metrics: perfMetrics,
	}, nil
}
