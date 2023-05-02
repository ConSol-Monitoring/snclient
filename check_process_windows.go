package snclient

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"internal/wmi"

	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_process"] = CheckEntry{"check_process", new(CheckProcess)}
}

type CheckProcess struct {
	noCopy noCopy
	data   CheckData
}

var ProcessStates = map[string]string{
	"stopped": "0",
	"started": "1",
	"0":       "stopped",
	"1":       "started",
}

/* check_process_windows
 * Description: Checks the state of a process on the host.
 * Thresholds: status
 * Units: ?
 */
func (l *CheckProcess) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(CheckExitOK)
	var output string
	l.data.detailSyntax = "%(process)=%(state)"
	l.data.topSyntax = "%(problem_list)"
	l.data.okSyntax = "all processes are ok."
	l.data.emptySyntax = "No processes found"
	l.data.emptyState = CheckExitUnknown
	argList, err := ParseArgs(args, &l.data)
	if err != nil {
		return nil, fmt.Errorf("args error: %s", err.Error())
	}
	var processes []string
	var checkData map[string]string

	// parse threshold args
	for _, arg := range argList {
		if arg.key == "process" {
			processes = append(processes, arg.value)
		}
	}

	metrics := make([]*CheckMetric, 0, len(processes))
	okList := make([]string, 0, len(processes))
	warnList := make([]string, 0, len(processes))
	critList := make([]string, 0, len(processes))
	processData, _, err := wmi.Query(`Select
									Name,
									CommandLine,
									CreationDate,
									ExecutablePath,
									HandleCount,
									KernelModeTime,
									PageFileUsage,
									PeakPageFileUsage,
									PeakVirtualSize,
									PeakWorkingSetSize,
									ProcessId,
									WorkingSetSize,
									VirtualSize,
									UserModeTime,
									ThreadCount
								From
									Win32_Process
								`)
	if err != nil {
		return nil, fmt.Errorf("wmi query failed: %s", err.Error())
	}
	runningProcs := map[string]map[string]string{}

	// collect process state
	for _, process := range processData {
		var name string
		for _, proc := range process {
			if proc.Key == "Name" {
				name = proc.Value
				runningProcs[name] = map[string]string{}
				if slices.Contains(processes, "*") {
					processes = append(processes, name)
				}
			}

			runningProcs[name][proc.Key] = proc.Value
		}
	}

	for _, process := range processes {
		proc, exists := runningProcs[process]
		if !exists {
			continue
		}

		var state float64

		if proc["ProcessId"] == "0" || proc["ThreadCount"] == "0" {
			state = 0
		} else {
			state = 1
		}

		metrics = append(metrics, &CheckMetric{Name: process, Value: state})
		dre := regexp.MustCompile(`\d+\.\d+`)
		mdata := map[string]string{
			"process":          process,
			"state":            ProcessStates[strconv.FormatFloat(state, 'f', 0, 64)],
			"command_line":     proc["CommandLine"],
			"creation":         dre.FindString(proc["CreationDate"]),
			"exe":              process,
			"filename":         proc["ExecutablePath"],
			"handles":          proc["HandleCount"],
			"kernel":           proc["KernelModeTime"],
			"pagefile":         proc["PageFileUsage"],
			"peak_pagefile":    proc["PeakPageFileUsage"],
			"peak_virtual":     proc["PeakVirtualSize"],
			"peak_working_set": proc["PeakWorkingSetSize"],
			"pid":              proc["ProcessId"],
			"user":             proc["UserModeTime"],
			"virtual":          proc["VirtualSize"],
			"working_set":      proc["WorkingSetSize"],
		}

		if CompareMetrics(mdata, l.data.critThreshold) && l.data.critThreshold.value != "none" {
			critList = append(critList, ParseSyntax(l.data.detailSyntax, mdata))

			continue
		}

		if CompareMetrics(mdata, l.data.warnThreshold) && l.data.warnThreshold.value != "none" {
			warnList = append(warnList, ParseSyntax(l.data.detailSyntax, mdata))

			continue
		}

		okList = append(okList, ParseSyntax(l.data.detailSyntax, mdata))
	}

	totalList := append(okList, append(warnList, critList...)...)

	if len(critList) > 0 {
		state = CheckExitCritical
	} else if len(warnList) > 0 {
		state = CheckExitWarning
	}

	if len(totalList) == 0 {
		state = l.data.emptyState
	}

	checkData = map[string]string{
		"state":        ProcessStates[strconv.FormatInt(state, 10)],
		"count":        strconv.FormatInt(int64(len(totalList)), 10),
		"ok_list":      strings.Join(okList, ", "),
		"warn_list":    strings.Join(warnList, ", "),
		"crit_list":    strings.Join(critList, ", "),
		"list":         strings.Join(totalList, ", "),
		"problem_list": strings.Join(append(critList, warnList...), ", "),
	}

	if state == CheckExitOK {
		output = ParseSyntax(l.data.okSyntax, checkData)
	} else {
		output = ParseSyntax(l.data.topSyntax, checkData)
	}

	return &CheckResult{
		State:   state,
		Output:  output,
		Metrics: metrics,
	}, nil
}
