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

type CheckMemory struct{}

/* check_memory
 * Description: Checks the memory usage on the host.
 * Thresholds: used, free, used_pct, free_pct
 * Units: B, KB, MB, GB, TB, %
 */
func (l *CheckMemory) Check(_ *Agent, args []string) (*CheckResult, error) {
	check := &CheckData{
		result: &CheckResult{
			State: CheckExitOK,
		},
		detailSyntax: "%(type) = %(used)",
		topSyntax:    "${status}: ${list}",
	}
	_, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	physical, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("fetching virtual memory failed: %s", err.Error())
	}

	swap, err := mem.SwapMemory()
	if err != nil {
		return nil, fmt.Errorf("fetching swap failed: %s", err.Error())
	}

	physicalM := map[string]string{
		"physical": fmt.Sprintf("%d", physical.Used),
		"type":     "physical",
		"used":     humanize.Bytes(physical.Used),
		"free":     humanize.Bytes(physical.Free),
		"size":     humanize.Bytes(physical.Total),
		"used_pct": strconv.FormatFloat(physical.UsedPercent, 'f', 0, 64),
		"free_pct": strconv.FormatUint(physical.Free*100/(physical.Total+1), 10),
	}
	check.listData = append(check.listData, physicalM)

	committedM := map[string]string{
		"page":     fmt.Sprintf("%d", physical.Used),
		"type":     "committed",
		"used":     humanize.Bytes(swap.Used),
		"free":     humanize.Bytes(swap.Free),
		"size":     humanize.Bytes(swap.Total),
		"used_pct": strconv.FormatFloat(swap.UsedPercent, 'f', 0, 64),
		"free_pct": strconv.FormatUint(swap.Free*100/(swap.Total+1), 10),
	}
	check.listData = append(check.listData, committedM)

	value := float64(physical.Used)
	size := float64(physical.Total)
	min := float64(0)
	check.result.Metrics = append(check.result.Metrics, &CheckMetric{
		Name:     "physical",
		Unit:     "B",
		Value:    value,
		Min:      &min,
		Max:      &size,
		Warning:  check.warnThreshold,
		Critical: check.critThreshold,
	})

	return check.Finalize()
}
