package snclient

import (
	"fmt"

	"pkg/humanize"

	"github.com/shirou/gopsutil/v3/mem"
	"golang.org/x/exp/slices"
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
	memType := []string{"committed", "physical"}
	check := &CheckData{
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]interface{}{
			"type": &memType,
		},
		defaultWarning:  "used > 80%",
		defaultCritical: "used > 90%",
		detailSyntax:    "%(type) = %(used)",
		topSyntax:       "${status}: ${list}",
	}
	_, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	check.SetDefaultThresholdUnit("%", []string{"used", "free"})
	check.ExpandThresholdUnit([]string{"k", "m", "g", "p", "e", "ki", "mi", "gi", "pi", "ei"}, "B", []string{"used", "free"})

	physical, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("fetching virtual memory failed: %s", err.Error())
	}

	swap, err := mem.SwapMemory()
	if err != nil {
		return nil, fmt.Errorf("fetching swap failed: %s", err.Error())
	}

	if physical.Total == 0 {
		return nil, fmt.Errorf("total memory is zero")
	}

	if slices.Contains(memType, "committed") {
		l.addMemType(check, "committed", swap.Used, swap.Free, swap.Total)
	}
	if slices.Contains(memType, "physical") {
		l.addMemType(check, "physical", physical.Used, physical.Free, physical.Total)
	}

	return check.Finalize()
}

func (l *CheckMemory) addMemType(check *CheckData, name string, used, free, total uint64) {
	entry := map[string]string{
		name:         fmt.Sprintf("%d", used),
		"type":       name,
		"used":       humanize.IBytesF(used, 2),
		"used_bytes": fmt.Sprintf("%d", used),
		"used_pct":   fmt.Sprintf("%.3f", (float64(used) * 100 / (float64(total)))),
		"free":       humanize.IBytesF(free*100/total, 2),
		"free_bytes": fmt.Sprintf("%d", free),
		"free_pct":   fmt.Sprintf("%.3f", (float64(free) * 100 / (float64(total)))),
		"size":       humanize.IBytesF(total, 2),
		"size_bytes": fmt.Sprintf("%d", total),
	}
	check.listData = append(check.listData, entry)
	if check.HasThreshold("free") {
		check.AddBytePercentMetrics("free", name, float64(free), float64(total))
	}
	if check.HasThreshold("used") {
		check.AddBytePercentMetrics("used", name, float64(used), float64(total))
	}
	if check.HasThreshold("free_pct") {
		check.AddPercentMetrics("free_pct", name+"_free_pct", float64(free), float64(total))
	}
	if check.HasThreshold("used_pct") {
		check.AddPercentMetrics("used_pct", name+"_used_pct", float64(free), float64(total))
	}
}
