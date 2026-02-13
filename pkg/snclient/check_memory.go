package snclient

import (
	"context"
	"fmt"
	"runtime"

	"github.com/consol-monitoring/snclient/pkg/humanize"
	"github.com/shirou/gopsutil/v4/mem"
)

func init() {
	AvailableChecks["check_memory"] = CheckEntry{"check_memory", NewCheckMemory}
}

type CheckMemory struct {
	memType CommaStringList
	// List the top N cpu consuming processes
	numProcs int64
	// Show arguments when listing the top N processes
	showArgs bool
}

func NewCheckMemory() CheckHandler {
	var memType CommaStringList
	switch runtime.GOOS {
	case "windows":
		memType = []string{"physical", "committed"}
	default:
		memType = []string{"physical", "swap"}
	}

	return &CheckMemory{
		memType: memType,
	}
}

func (l *CheckMemory) Build() *CheckData {
	return &CheckData{
		name: "check_memory",
		description: `Checks the memory usage on the host.

There are several types of memory that can be checked:

    physical: physical memory
    swap: swap memory (pagefile on windows)
    committed: committed memory as shown in the windows task manager (windows only, basically this is the physical + swap)
    virtual: available windows virtual address space (windows only)

read more on windows virtual address space:

    https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/ns-sysinfoapi-memorystatusex`,
		implemented:  ALL,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"type":            {value: &l.memType, description: "Type of memory to check. Default: physical,committed (win) or physical,swap (other)"},
			"n|procs-to-show": {value: &l.numProcs, description: "Number of processes to show when printing the top consuming processes"},
			"show-args":       {value: &l.showArgs, description: "Show arguments when listing the top N processes"},
		},
		defaultWarning:  "used > 80%",
		defaultCritical: "used > 90%",
		detailSyntax:    "%(type) = %(used)/%(size) (%(used_pct | fmt=%.1f )%)",
		topSyntax:       "%(status) - ${list}",
		attributes: []CheckAttribute{
			{name: "<type>", description: "used bytes with the type as key", unit: UByte},
			{name: "type", description: "checked type, either 'physical' or 'committed', 'swap' or 'virtual' (windows only)"},
			{name: "used", description: "Used memory in human readable bytes (IEC)", unit: UByte},
			{name: "used_bytes", description: "Used memory in bytes (IEC)", unit: UByte},
			{name: "used_pct", description: "Used memory in percent", unit: UPercent},
			{name: "free", description: "Free memory in human readable bytes (IEC)", unit: UByte},
			{name: "free_bytes", description: "Free memory in bytes (IEC)", unit: UByte},
			{name: "free_pct", description: "Free memory in percent", unit: UPercent},
			{name: "size", description: "Total memory in human readable bytes (IEC)", unit: UByte},
			{name: "size_bytes", description: "Total memory in bytes (IEC)", unit: UByte},
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

func (l *CheckMemory) Check(ctx context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	check.SetDefaultThresholdUnit("%", []string{"used", "free"})

	physical, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("fetching virtual memory failed: %s", err.Error())
	}

	if physical.Total == 0 {
		return nil, fmt.Errorf("total physical memory is zero")
	}

	if l.numProcs > 0 {
		err = appendProcs(ctx, check, l.numProcs, l.showArgs, "mem")
		if err != nil {
			return nil, fmt.Errorf("procs: %s", err.Error())
		}
	}

	for _, memType := range l.memType {
		switch memType {
		case "physical":
			l.addMemType(check, "physical", physical.Used, physical.Free, physical.Total)
		case "swap":
			swap, err := mem.SwapMemory()
			if err != nil {
				return nil, fmt.Errorf("fetching swap failed: %s", err.Error())
			}
			// osx changes swap total on demand, so always return something in inventory mode
			if swap.Total > 0 || check.hasArgsSupplied["type"] || (runtime.GOOS == "darwin" && check.output == OutputInventory) {
				l.addMemType(check, "swap", swap.Used, swap.Free, swap.Total)
			}
		case "committed":
			total, avail, err := l.committedMemory()
			if err != nil {
				return nil, err
			}
			if total > 0 || check.hasArgsSupplied["type"] {
				l.addMemType(check, "committed", total-avail, avail, total)
			}
		case "virtual":
			if runtime.GOOS != "windows" {
				return nil, fmt.Errorf("virtual memory is only supported on windows")
			}
			virtTotal, virtAvail, err := l.virtualMemory()
			if err != nil {
				return nil, err
			}
			l.addMemType(check, "virtual", virtTotal-virtAvail, virtAvail, virtTotal)
		default:
			return nil, fmt.Errorf("unknown type, please use 'physical',  'committed', 'swap' or 'virtual'")
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
	// avoid NaN values in percentages when total is zero
	if total == 0 {
		entry["used_pct"] = "0"
		entry["free_pct"] = "0"
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
