package snclient

import (
	"context"
	"fmt"

	"pkg/humanize"

	"github.com/shirou/gopsutil/v3/mem"
	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_memory"] = CheckEntry{"check_memory", new(CheckMemory)}
}

type CheckMemory struct {
	memType []string
}

func (l *CheckMemory) Build() *CheckData {
	l.memType = []string{"committed", "physical"}

	return &CheckData{
		name:        "check_memory",
		description: "Checks the memory usage on the host.",
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]interface{}{
			"type": &l.memType,
		},
		defaultWarning:  "used > 80%",
		defaultCritical: "used > 90%",
		detailSyntax:    "%(type) = %(used)",
		topSyntax:       "${status}: ${list}",
	}
}

func (l *CheckMemory) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
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
		return nil, fmt.Errorf("total physical memory is zero")
	}

	if slices.Contains(l.memType, "committed") && swap.Total > 0 {
		l.addMemType(check, "committed", swap.Used, swap.Free, swap.Total)
	}
	if slices.Contains(l.memType, "physical") {
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
