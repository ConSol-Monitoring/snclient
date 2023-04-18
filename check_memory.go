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
	data   CheckData
}

/* check_memory
 * Description: Checks the memory usage on the host.
 * Tresholds: used, free, used_pct, free_pct
 * Units: B, KB, MB, GB, TB, %
 */
func (l *CheckMemory) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(0)
	l.data.detailSyntax = "%(type) = %(used)"
	ParseArgs(args, &l.data)
	var checkData []map[string]string

	// collect ram metrics (physical + committed)
	physical, _ := mem.VirtualMemory()
	committed, _ := mem.SwapMemory()

	physicalM := []MetricData{
		{name: "used", value: strconv.FormatUint(physical.Used, 10)},
		{name: "free", value: strconv.FormatUint(physical.Free, 10)},
		{name: "used_pct", value: strconv.FormatFloat(physical.UsedPercent, 'f', 0, 64)},
		{name: "free_pct", value: strconv.FormatUint(physical.Free*100/physical.Total, 10)},
	}

	checkData = append(checkData, map[string]string{
		"type":     "physical",
		"used":     humanize.Bytes(physical.Used),
		"free":     humanize.Bytes(physical.Free),
		"size":     humanize.Bytes(physical.Total),
		"used_pct": strconv.FormatFloat(physical.UsedPercent, 'f', 0, 64),
		"free_pct": strconv.FormatUint(physical.Free*100/physical.Total, 10),
	})

	committedM := []MetricData{
		{name: "used", value: strconv.FormatUint(committed.Used, 10)},
		{name: "free", value: strconv.FormatUint(committed.Free, 10)},
		{name: "used_pct", value: strconv.FormatFloat(committed.UsedPercent, 'f', 0, 64)},
		{name: "free_pct", value: strconv.FormatUint(committed.Free*100/committed.Total, 10)},
	}

	checkData = append(checkData, map[string]string{
		"type":     "committed",
		"used":     humanize.Bytes(committed.Used),
		"free":     humanize.Bytes(committed.Free),
		"size":     humanize.Bytes(committed.Total),
		"used_pct": strconv.FormatFloat(committed.UsedPercent, 'f', 0, 64),
		"free_pct": strconv.FormatUint(committed.Free*100/committed.Total, 10),
	})

	// compare ram metrics to tresholds
	if CompareMetrics(physicalM, l.data.warnTreshold) || CompareMetrics(committedM, l.data.warnTreshold) {
		state = CheckExitWarning
	}

	if CompareMetrics(physicalM, l.data.critTreshold) || CompareMetrics(committedM, l.data.critTreshold) {
		state = CheckExitCritical
	}

	output := ""
	for i, d := range checkData {
		output += ParseSyntax(l.data.detailSyntax, d)
		if i != len(checkData)-1 {
			output += ", "
		}
	}

	// build perfdata
	metrics := []*CheckMetric{}

	for _, m := range committedM {
		if m.name == l.data.warnTreshold.name || m.name == l.data.critTreshold.name {
			value, _ := strconv.ParseFloat(m.value, 64)
			metrics = append(metrics, &CheckMetric{
				Name:  fmt.Sprintf("committed_%v", l.data.warnTreshold.name),
				Unit:  "",
				Value: value,
			})
		}
	}

	for _, m := range physicalM {
		if m.name == l.data.warnTreshold.name || m.name == l.data.critTreshold.name {
			value, _ := strconv.ParseFloat(m.value, 64)
			metrics = append(metrics, &CheckMetric{
				Name:  fmt.Sprintf("physical_%v", l.data.warnTreshold.name),
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
