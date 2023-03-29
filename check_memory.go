package snclient

import (
	"fmt"
	"strconv"

	"github.com/dustin/go-humanize"
	"github.com/shirou/gopsutil/v3/mem"
)

func init() {
	AvailableChecks["check_memory"] = CheckEntry{"check_memory", new(CheckMemory)}
}

type CheckMemory struct {
	noCopy noCopy
}

/* check_memory todo
 * todo .
 */
func (l *CheckMemory) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(0)
	argList := ParseArgs(args)
	var warnTreshold Treshold
	var critTreshold Treshold

	// parse treshold args
	for _, arg := range argList {
		switch arg.key {
		case "warn", "warning":
			warnTreshold = ParseTreshold(arg.value)
		case "crit", "critical":
			critTreshold = ParseTreshold(arg.value)
		}
	}

	// collect ram metrics (physical + committed)
	physical, _ := mem.VirtualMemory()
	committed, _ := mem.SwapMemory()

	physicalM := []MetricData{
		{name: "used", value: strconv.FormatUint(physical.Used, 10)},
		{name: "free", value: strconv.FormatUint(physical.Free, 10)},
		{name: "used_pct", value: strconv.FormatFloat(physical.UsedPercent, 'f', 0, 64)},
		{name: "free_pct", value: strconv.FormatUint(physical.Free*100/physical.Total, 10)},
	}

	committedM := []MetricData{
		{name: "used", value: strconv.FormatUint(committed.Used, 10)},
		{name: "free", value: strconv.FormatUint(committed.Free, 10)},
		{name: "used_pct", value: strconv.FormatFloat(committed.UsedPercent, 'f', 0, 64)},
		{name: "free_pct", value: strconv.FormatUint(committed.Free*100/committed.Total, 10)},
	}

	// compare ram metrics to tresholds
	if CompareMetrics(physicalM, warnTreshold) || CompareMetrics(committedM, warnTreshold) {
		state = CheckExitWarning
	}

	if CompareMetrics(physicalM, critTreshold) || CompareMetrics(committedM, critTreshold) {
		state = CheckExitCritical
	}

	output := fmt.Sprintf("committed = %v, physical = %v", humanize.Bytes(committed.Used), humanize.Bytes(physical.Used))

	// build perfdata
	metrics := []*CheckMetric{}

	for _, m := range committedM {
		if m.name == warnTreshold.name {
			value, _ := strconv.ParseFloat(m.value, 64)
			metrics = append(metrics, &CheckMetric{
				Name:  fmt.Sprintf("committed_%v", warnTreshold.name),
				Unit:  "",
				Value: value,
			})
		}
	}

	for _, m := range physicalM {
		if m.name == warnTreshold.name {
			value, _ := strconv.ParseFloat(m.value, 64)
			metrics = append(metrics, &CheckMetric{
				Name:  fmt.Sprintf("physical_%v", warnTreshold.name),
				Unit:  "",
				Value: value,
			})
		}
	}

	return &CheckResult{
		State:   state,
		Output:  output,
		Metrics: metrics,
	}, nil
}
