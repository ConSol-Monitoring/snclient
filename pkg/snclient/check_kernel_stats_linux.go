package snclient

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pkg/convert"

	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_kernel_stats"] = CheckEntry{"check_kernel_stats", NewCheckKernelStats}
}

const (
	KernelRateDuration = 30 * time.Second
)

type CheckKernelStats struct {
	snc   *Agent
	types []string
}

func NewCheckKernelStats() CheckHandler {
	return &CheckKernelStats{}
}

func (l *CheckKernelStats) Build() *CheckData {
	return &CheckData{
		name:         "check_kernel_stats",
		description:  "Checks some metrics of the linux kernel. Currently support context switches, process creations and total number of threads.",
		implemented:  Linux,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"type": {value: &l.types, description: "Select metric type to show, can be: ctxt, processes or threads"},
		},
		defaultWarning:  "threads > 8000",
		defaultCritical: "threads > 10000",
		okSyntax:        "%(status): %(list)",
		detailSyntax:    "%(label) %(human)",
		topSyntax:       "%(status): %(list)",
		emptySyntax:     "%(status): No metrics found",
		emptyState:      CheckExitUnknown,
		attributes: []CheckAttribute{
			{name: "name", description: "Name of the metric"},
			{name: "label", description: "Label of the metric"},
			{name: "rate", description: "Rate of this metric"},
			{name: "current", description: "Current raw value"},
			{name: "human", description: "Human readable number"},
		},
		exampleDefault: `
    check_kernel_stats
    OK: Context Switches 29.2/s, Process Creations 12.7/s, Threads 2574 |'ctxt'=29.2/s 'processes'=12.7/s 'threads'=2574;8000;10000;0
	`,
	}
}

func (l *CheckKernelStats) Check(_ context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	l.snc = snc

	if len(l.types) == 0 || slices.Contains(l.types, "ctxt") {
		ctxt, current := l.getRate("ctxt")
		entry := map[string]string{
			"name":    "ctxt",
			"label":   "Context Switches",
			"rate":    fmt.Sprintf("%f", ctxt),
			"current": fmt.Sprintf("%f", current),
			"human":   fmt.Sprintf("%.1f/s", ctxt),
		}
		check.listData = append(check.listData, entry)
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			Name:     "ctxt",
			Value:    ctxt,
			Unit:     "/s",
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
			Min:      &Zero,
		})
	}
	if len(l.types) == 0 || slices.Contains(l.types, "processes") {
		processes, current := l.getRate("processes")
		entry := map[string]string{
			"name":    "processes",
			"label":   "Process Creations",
			"rate":    fmt.Sprintf("%f", processes),
			"current": fmt.Sprintf("%f", current),
			"human":   fmt.Sprintf("%.1f/s", processes),
		}
		check.listData = append(check.listData, entry)
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			Name:     "processes",
			Value:    processes,
			Unit:     "/s",
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
			Min:      &Zero,
		})
	}

	if len(l.types) == 0 || slices.Contains(l.types, "processes") {
		threads := l.getNumThreads()
		entry := map[string]string{
			"name":    "threads",
			"label":   "Threads",
			"rate":    "",
			"current": fmt.Sprintf("%d", threads),
			"human":   fmt.Sprintf("%d", threads),
		}
		check.listData = append(check.listData, entry)
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			Name:     "threads",
			Value:    threads,
			Unit:     "",
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
			Min:      &Zero,
		})
	}

	return check.Finalize()
}

func (l *CheckKernelStats) getRate(name string) (rate, last float64) {
	rate, _ = l.snc.Counter.GetRate("kernel", name, KernelRateDuration)
	lastC := l.snc.Counter.Get("kernel", name)
	if lastC != nil {
		lastV := lastC.GetLast()
		if lastV != nil {
			last = lastV.value
		}
	}

	if rate < 0 {
		rate = 0
	}

	return
}

func (l *CheckKernelStats) getNumThreads() (num int64) {
	files, _ := filepath.Glob("/proc/*/status")
	for _, file := range files {
		data, _ := os.ReadFile(file)
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			switch fields[0] {
			case "Threads:":
				num += convert.Int64(fields[1])
			default:
			}
		}
	}

	return
}