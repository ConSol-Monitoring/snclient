package snclient

import (
	"fmt"
	"strconv"

	"github.com/dustin/go-humanize"
	"github.com/shirou/gopsutil/v3/mem"
)

func init() {
	AvailableChecks["check_memory"] = CheckEntry{"check_memory", new(check_memory)}
}

type check_memory struct {
	noCopy noCopy
}

/* check_memory todo
 * todo
 */
func (l *check_memory) Check(args []string) (*CheckResult, error) {

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
	v, _ := mem.VirtualMemory()
	s, _ := mem.SwapMemory()

	physicalM := []MetricData{
		MetricData{name: "used", value: strconv.FormatUint(v.Used, 10)},
		MetricData{name: "free", value: strconv.FormatUint(v.Free, 10)},
		MetricData{name: "used_pct", value: strconv.FormatFloat(v.UsedPercent, 'f', 0, 64)},
		MetricData{name: "free_pct", value: strconv.FormatUint(v.Free*100/v.Total, 10)},
	}

	committedM := []MetricData{
		MetricData{name: "used", value: strconv.FormatUint(s.Used, 10)},
		MetricData{name: "free", value: strconv.FormatUint(s.Free, 10)},
		MetricData{name: "used_pct", value: strconv.FormatFloat(s.UsedPercent, 'f', 0, 64)},
		MetricData{name: "free_pct", value: strconv.FormatUint(s.Free*100/s.Total, 10)},
	}

	// compare ram metrics to tresholds
	if CompareMetrics(physicalM, warnTreshold) || CompareMetrics(committedM, warnTreshold) {
		state = CheckExitWarning
	}

	if CompareMetrics(physicalM, critTreshold) || CompareMetrics(committedM, critTreshold) {
		state = CheckExitCritical
	}

	output := fmt.Sprintf("committed = %v, physical = %v", humanize.Bytes(s.Used), humanize.Bytes(v.Used))

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
