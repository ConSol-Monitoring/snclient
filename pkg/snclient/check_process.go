package snclient

import (
	"context"
	"fmt"

	"github.com/consol-monitoring/snclient/pkg/convert"
)

func init() {
	AvailableChecks["check_process"] = CheckEntry{"check_process", NewCheckProcess}
}

type CheckProcess struct {
	processes   []string
	timeZoneStr string
}

func NewCheckProcess() CheckHandler {
	return &CheckProcess{
		timeZoneStr: "Local",
	}
}

func (l *CheckProcess) Build() *CheckData {
	return &CheckData{
		name:         "check_process",
		description:  "Checks the state and metrics of one or multiple processes.",
		implemented:  ALL,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"process":  {value: &l.processes, description: "The process to check, set to * to check all. Default: *", isFilter: true},
			"timezone": {value: &l.timeZoneStr, description: "Sets the timezone for time metrics (default is local time)"},
		},
		okSyntax:     "%(status) - all %{count} processes are ok.",
		detailSyntax: "${exe}=${state}",
		topSyntax:    "%(status) - ${problem_list}",
		emptyState:   3,
		emptySyntax:  "%(status) - check_process failed to find anything with this filter.",
		attributes: []CheckAttribute{
			{name: "process", description: "Name of the executable (without path)"},
			{name: "exe", description: "Name of the executable (without path)"},
			{name: "filename", description: "Name of the executable with path"},
			{name: "command_line", description: "Full command line of process"},
			{name: "state", description: "Current state (windows: started, stopped, hung - linux: idle, lock, running, sleep, stop, wait and zombie)"},
			{name: "creation", description: "Start time of process"},
			{name: "pid", description: "Process id"},
			{name: "uid", description: "User if of process owner (linux only)"},
			{name: "username", description: "User name of process owner (linux only)"},
			{name: "cpu", description: "CPU usage in percent"},
			{name: "virtual", description: "Virtual memory usage in bytes"},
			{name: "rss", description: "Resident memory usage in bytes"},
			{name: "pagefile", description: "Swap memory usage in bytes"},
			{name: "oldest", description: "Unix timestamp of oldest process"},
			{name: "peak_pagefile", description: "Peak swap memory usage in bytes (windows only)"},
			{name: "handles", description: "Number of handles (windows only)"},
			{name: "kernel", description: "Kernel time in seconds (windows only)"},
			{name: "peak_virtual", description: "Peak virtual size in bytes (windows only)"},
			{name: "peak_working_set", description: "Peak working set in bytes (windows only)"},
			{name: "user", description: "User time in seconds (windows only)"},
			{name: "working_set", description: "Working set in bytes (windows only)"},
		},
		exampleDefault: `
    check_process
    OK - 417 processes. |'count'=417;;;0

Check specific process by name (adding some metrics as well)

    check_process \
        process=httpd \
        warn='count < 1 || count > 10' \
        crit='count < 0 || count > 20' \
        top-syntax='%{status} - %{count} processes, memory %{rss|h}B, cpu %{cpu:fmt=%.1f}%, started %{oldest:age|duration} ago'
    WARNING - 12 processes, memory 62.58 MB, started 01:11h ago |...

If zero is a valid threshold, set the empty-state to ok

    check_process process=qemu warn='count <= 0 || count > 10' crit='count <= 0 || count > 20' empty-state=0
    OK - check_process failed to find anything with this filter.
	`,
		exampleArgs: `warn='count <= 0 || count > 10' crit='count <= 0 || count > 20'`,
	}
}

func (l *CheckProcess) Check(ctx context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	err := l.fetchProcs(ctx, check)
	if err != nil {
		return nil, err
	}

	check.ExpandThresholdUnit([]string{"k", "m", "g", "p", "e", "ki", "mi", "gi", "pi", "ei"}, "B", []string{"rss", "virtual", "pagefile"})

	check.listData = check.Filter(check.filter, check.listData)
	check.result.Metrics = append(check.result.Metrics, &CheckMetric{
		Name:     "count",
		Value:    len(check.listData),
		Min:      &Zero,
		Warning:  check.warnThreshold,
		Critical: check.critThreshold,
	})

	totalRss := int64(0)
	totalVirtual := int64(0)
	totalCPU := float64(0)
	oldest := int64(-1)
	youngest := int64(0)
	for _, p := range check.listData {
		totalCPU += convert.Float64(p["cpu"])
		totalRss += convert.Int64(p["rss"])
		totalVirtual += convert.Int64(p["virtual"])
		create := convert.Int64(p["creation_unix"])
		if create < youngest {
			youngest = create
		}
		if oldest == -1 || oldest > create {
			oldest = create
		}
	}
	check.details = map[string]string{
		"cpu":      fmt.Sprintf("%f", totalCPU),
		"rss":      fmt.Sprintf("%d", totalRss),
		"virtual":  fmt.Sprintf("%d", totalVirtual),
		"oldest":   fmt.Sprintf("%d", oldest),
		"youngest": fmt.Sprintf("%d", youngest),
	}

	if check.HasThreshold("rss") || len(l.processes) > 0 || len(check.filter) > 0 {
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			Name:     "rss",
			Unit:     "B",
			Value:    totalRss,
			Min:      &Zero,
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
		})
	}

	if check.HasThreshold("virtual") || len(l.processes) > 0 || len(check.filter) > 0 {
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			Name:     "virtual",
			Unit:     "B",
			Value:    totalVirtual,
			Min:      &Zero,
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
		})
	}

	if check.HasThreshold("cpu") || len(l.processes) > 0 || len(check.filter) > 0 {
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			Name:     "cpu",
			Unit:     "%",
			Value:    totalCPU,
			Min:      &Zero,
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
		})
	}

	return check.Finalize()
}
