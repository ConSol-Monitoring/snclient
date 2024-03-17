package snclient

import (
	"context"
	"fmt"

	"pkg/humanize"

	"github.com/shirou/gopsutil/v3/mem"
)

func init() {
	AvailableChecks["check_memory"] = CheckEntry{"check_memory", NewCheckMemory}
}

type CheckMemory struct {
	memType []string
}

func NewCheckMemory() CheckHandler {
	return &CheckMemory{
		memType: []string{"physical", "committed"},
	}
}

func (l *CheckMemory) Build() *CheckData {
	return &CheckData{
		name:         "check_memory",
		description:  "Checks the memory usage on the host.",
		implemented:  ALL,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"type": {value: &l.memType, description: "Type of memory to check. Default: physical,committed"},
		},
		defaultWarning:  "used > 80%",
		defaultCritical: "used > 90%",
		detailSyntax:    "%(type) = %(used)/%(size) (%(used_pct | fmt=%.1f )%)",
		topSyntax:       "%(status) - ${list}",
		attributes: []CheckAttribute{
			{name: "<type>", description: "used bytes with the type as key"},
			{name: "type", description: "checked type, either 'physical' or 'committed' (swap)"},
			{name: "used", description: "Used memory in human readable bytes (IEC)"},
			{name: "used_bytes", description: "Used memory in bytes (IEC)"},
			{name: "used_pct", description: "Used memory in percent"},
			{name: "free", description: "Free memory in human readable bytes (IEC)"},
			{name: "free_bytes", description: "Free memory in bytes (IEC)"},
			{name: "free_pct", description: "Free memory in percent"},
			{name: "size", description: "Total memory in human readable bytes (IEC)"},
			{name: "size_bytes", description: "Total memory in bytes (IEC)"},
		},
		exampleDefault: `
    check_memory
    OK - physical = 6.98 GiB, committed = 719.32 MiB|...

Changing the return syntax to get more information:

    check_memory 'top-syntax=${list}' 'detail-syntax=${type} free: ${free} used: ${used} size: ${size}'
    physical free: 35.00 B used: 7.01 GiB size: 31.09 GiB, committed free: 27.00 B used: 705.57 MiB size: 977.00 MiB |...
	`,
		exampleArgs: `'warn=used > 80%' 'crit=used > 90%'`,
	}
}

func (l *CheckMemory) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	check.SetDefaultThresholdUnit("%", []string{"used", "free"})
	check.ExpandThresholdUnit([]string{"k", "m", "g", "p", "e", "ki", "mi", "gi", "pi", "ei"}, "B", []string{"used", "free"})

	physical, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("fetching virtual memory failed: %s", err.Error())
	}

	if physical.Total == 0 {
		return nil, fmt.Errorf("total physical memory is zero")
	}

	for _, memType := range l.memType {
		switch memType {
		case "physical":
			l.addMemType(check, "physical", physical.Used, physical.Free, physical.Total)
		case "committed":
			swap, err := mem.SwapMemory()
			if err != nil {
				return nil, fmt.Errorf("fetching swap failed: %s", err.Error())
			}
			if swap.Total > 0 {
				l.addMemType(check, "committed", swap.Used, swap.Free, swap.Total)
			}
		case "virtual":
			l.addMemType(check, "virtual", physical.VirtualTotal-physical.VirtualAvail, physical.VirtualAvail, physical.VirtualTotal)
		default:
			return nil, fmt.Errorf("unknown type, please use 'physical' or 'committed'")
		}
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
		"free":       humanize.IBytesF(free, 2),
		"free_bytes": fmt.Sprintf("%d", free),
		"free_pct":   fmt.Sprintf("%.3f", (float64(free) * 100 / (float64(total)))),
		"size":       humanize.IBytesF(total, 2),
		"size_bytes": fmt.Sprintf("%d", total),
	}
	check.listData = append(check.listData, entry)
	if check.HasThreshold("free") || check.HasThreshold("free_pct") {
		check.warnThreshold = check.TransformMultipleKeywords([]string{"free_pct"}, "free", check.warnThreshold)
		check.critThreshold = check.TransformMultipleKeywords([]string{"free_pct"}, "free", check.critThreshold)
		check.AddBytePercentMetrics("free", name+"_free", float64(free), float64(total))
	}
	if check.HasThreshold("used") || check.HasThreshold("used_pct") {
		check.warnThreshold = check.TransformMultipleKeywords([]string{"used_pct"}, "used", check.warnThreshold)
		check.critThreshold = check.TransformMultipleKeywords([]string{"used_pct"}, "used", check.critThreshold)
		check.AddBytePercentMetrics("used", name, float64(used), float64(total))
	}
}
