package snclient

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"pkg/wmi"

	"golang.org/x/exp/slices"
)

var ProcessStates = map[string]string{
	"stopped": "0",
	"started": "1",
	"0":       "stopped",
	"1":       "started",
}

func (l *CheckProcess) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	query := `
		Select
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
	`
	processData, err := wmi.QueryDefaultRetry(query)
	if err != nil {
		return nil, fmt.Errorf("wmi query failed: %s", err.Error())
	}
	runningProcs := map[string]map[string]string{}

	// collect process state
	for _, process := range processData {
		name := process["Name"]
		runningProcs[name] = process
	}

	if slices.Contains(l.processes, "*") {
		for _, process := range processData {
			l.processes = append(l.processes, process["Name"])
		}
	}

	for _, process := range l.processes {
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
