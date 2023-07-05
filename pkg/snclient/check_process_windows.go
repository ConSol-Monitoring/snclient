package snclient

import (
	"fmt"
	"regexp"
	"strconv"

	"pkg/wmi"

	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_process"] = CheckEntry{"check_process", new(CheckProcess)}
}

type CheckProcess struct{}

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
func (l *CheckProcess) Check(_ *Agent, args []string) (*CheckResult, error) {
	check := &CheckData{
		result: &CheckResult{
			State: CheckExitOK,
		},
		detailSyntax: "%(process)=%(state)",
		topSyntax:    "%(status): %(problem_list)",
		okSyntax:     "%(status): ${list}",
		emptySyntax:  "No processes found",
		emptyState:   CheckExitUnknown,
	}
	argList, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	var processes []string

	// parse threshold args
	for _, arg := range argList {
		if arg.key == "process" {
			processes = append(processes, arg.value)
		}
	}

	processData, err := wmi.Query(`Select
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

		dre := regexp.MustCompile(`\d+\.\d+`)
		check.listData = append(check.listData, map[string]string{
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
		})
	}

	return check.Finalize()
}
