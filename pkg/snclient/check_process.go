package snclient

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
			"process":  {value: &l.processes, description: "The process to check, set to * to check all. Default: *"},
			"timezone": {value: &l.timeZoneStr, description: "Sets the timezone for time metrics (default is local time)"},
		},
		okSyntax:     "%(status): all processes are ok.",
		detailSyntax: "${exe}=${state}",
		topSyntax:    "${status}: ${problem_list}",
		emptyState:   3,
		emptySyntax:  "check_process failed to find anything with this filter.",
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
			{name: "virtual", description: "Virtual memory usage in bytes"},
			{name: "rss", description: "Resident memory usage in bytes (linux only)"},
			{name: "pagefile", description: "Swap memory usage in bytes"},
			{name: "peak_pagefile", description: "Peak swap memory usage in bytes (windows only)"},
			{name: "handles", description: "Number of handles (windows only)"},
			{name: "kernel", description: "Kernel time in seconds (windows only)"},
			{name: "peak_virtual", description: "Peak virtual size in bytes (windows only)"},
			{name: "peak_working_set", description: "Peak working set in bytes (windows only)"},
			{name: "user", description: "User time in seconds (windows only)"},
			{name: "working_set", description: "Working set in bytes (windows only)"},
		},
		exampleDefault: `
    check_process process=explorer.exe
    OK: explorer.exe=started
	`,
		exampleArgs: `'warn=used > 80%' 'crit=used > 95%'`,
	}
}
