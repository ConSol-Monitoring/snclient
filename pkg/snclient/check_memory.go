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
	data   *CheckData
}

/* check_memory
 * Description: Checks the memory usage on the host.
 * Thresholds: used, free, used_pct, free_pct
 * Units: B, KB, MB, GB, TB, %
 */
func (l *CheckMemory) Check(_ *Agent, args []string) (*CheckResult, error) {
	l.data = &CheckData{
		result: CheckResult{
			State: CheckExitOK,
		},
		detailSyntax: "%(type) = %(used)",
		topSyntax:    "${status}: ${list}",
	}
	_, err := ParseArgs(args, l.data)
	if err != nil {
		return nil, err
	}

	physical, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("fetching virtual memory failed: %s", err.Error())
	}

	committed, err := mem.SwapMemory()
	if err != nil {
		return nil, fmt.Errorf("fetching swap failed: %s", err.Error())
	}

	physicalM := map[string]string{
		"type":     "physical",
		"used":     humanize.Bytes(physical.Used),
		"free":     humanize.Bytes(physical.Free),
		"size":     humanize.Bytes(physical.Total),
		"used_pct": strconv.FormatFloat(physical.UsedPercent, 'f', 0, 64),
		"free_pct": strconv.FormatUint(physical.Free*100/(physical.Total+1), 10),
	}
	l.data.listData = append(l.data.listData, physicalM)

	committedM := map[string]string{
		"type":     "committed",
		"used":     humanize.Bytes(committed.Used),
		"free":     humanize.Bytes(committed.Free),
		"size":     humanize.Bytes(committed.Total),
		"used_pct": strconv.FormatFloat(committed.UsedPercent, 'f', 0, 64),
		"free_pct": strconv.FormatUint(committed.Free*100/(committed.Total+1), 10),
	}
	l.data.listData = append(l.data.listData, committedM)

	l.data.Check(CheckExitCritical, l.data.critThreshold, physicalM)
	l.data.Check(CheckExitCritical, l.data.critThreshold, committedM)
	l.data.Check(CheckExitWarning, l.data.warnThreshold, physicalM)
	l.data.Check(CheckExitWarning, l.data.warnThreshold, committedM)

	value := float64(physical.Used)
	size := float64(physical.Total)
	min := float64(0)
	l.data.result.Metrics = append(l.data.result.Metrics, &CheckMetric{
		Name:     "physical",
		Unit:     "B",
		Value:    value,
		Min:      &min,
		Max:      &size,
		Warning:  l.data.warnThreshold,
		Critical: l.data.critThreshold,
	})

	return &l.data.result, nil
}
