package snclient

import (
	"context"
	"fmt"
	"time"

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
		description:  "Checks the metrics of the linux kernel.",
		implemented:  Linux,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"type": {value: &l.types, description: "Select metric type to show, can be: ctxt or processes"},
		},
		okSyntax:     "%(status): %(list)",
		detailSyntax: "%(label) %(rate:fmt=%.1f)/s",
		topSyntax:    "%(status): %(list)",
		emptySyntax:  "%(status): No metrics found",
		emptyState:   CheckExitUnknown,
		attributes: []CheckAttribute{
			{name: "name", description: "Name of the metric"},
			{name: "label", description: "Label of the metric"},
			{name: "rate", description: "Rate of this metric"},
			{name: "current", description: "Current raw value"},
		},
		exampleDefault: `
    check_kernel_stats
    OK: Context Switches 29.2/s, Process Creations 12.7/s |'ctxt'=29.2/s 'processes'=12.7/s
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
		}
		check.listData = append(check.listData, entry)
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			Name:     "ctxt",
			Value:    ctxt,
			Unit:     "/s",
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
		})
	}
	if len(l.types) == 0 || slices.Contains(l.types, "processes") {
		processes, current := l.getRate("processes")
		entry := map[string]string{
			"name":    "processes",
			"label":   "Process Creations",
			"rate":    fmt.Sprintf("%f", processes),
			"current": fmt.Sprintf("%f", current),
		}
		check.listData = append(check.listData, entry)
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			Name:     "processes",
			Value:    processes,
			Unit:     "/s",
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
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
