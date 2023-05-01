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
 * Thresholds: used, free, used_pct, free_pct
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

	physicalM := map[string]string{
		"type":     "physical",
		"used":     humanize.Bytes(physical.Used),
		"free":     humanize.Bytes(physical.Free),
		"size":     humanize.Bytes(physical.Total),
		"used_pct": strconv.FormatFloat(physical.UsedPercent, 'f', 0, 64),
		"free_pct": strconv.FormatUint(physical.Free*100/(physical.Total+1), 10),
	}

	checkData = append(checkData, physicalM)

	committedM := map[string]string{
		"type":     "committed",
		"used":     humanize.Bytes(committed.Used),
		"free":     humanize.Bytes(committed.Free),
		"size":     humanize.Bytes(committed.Total),
		"used_pct": strconv.FormatFloat(committed.UsedPercent, 'f', 0, 64),
		"free_pct": strconv.FormatUint(committed.Free*100/(committed.Total+1), 10),
	}

	checkData = append(checkData, committedM)

	// compare ram metrics to thresholds
	if CompareMetrics(physicalM, l.data.warnThreshold) || CompareMetrics(committedM, l.data.warnThreshold) {
		state = CheckExitWarning
	}

	if CompareMetrics(physicalM, l.data.critThreshold) || CompareMetrics(committedM, l.data.critThreshold) {
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

	for _, val := range committedM {
		value, _ := strconv.ParseFloat(val, 64)
		metrics = append(metrics, &CheckMetric{
			Name:  fmt.Sprintf("committed_%v", l.data.warnThreshold.name),
			Unit:  "",
			Value: value,
		})
	}

	for _, val := range physicalM {
		value, _ := strconv.ParseFloat(val, 64)
		metrics = append(metrics, &CheckMetric{
			Name:  fmt.Sprintf("physical_%v", l.data.warnThreshold.name),
			Unit:  "",
			Value: value,
		})
	}

	return &CheckResult{
		State:   state,
		Output:  output,
		Metrics: metrics,
	}, nil
}
