package snclient

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"pkg/wmi"

	"golang.org/x/exp/slices"
)

var ProcessStates = map[string]string{
	"stopped": "0",
	"started": "1",
	"0":       "stopped",
	"1":       "started",
}

// https://learn.microsoft.com/en-us/windows/win32/cimwin32prov/win32-process
type winProcess struct {
	Name               string
	CommandLine        string
	CreationDate       time.Time
	ExecutablePath     string
	HandleCount        uint32
	KernelModeTime     uint64
	PageFileUsage      uint32
	PeakPageFileUsage  uint32
	PeakVirtualSize    uint64
	PeakWorkingSetSize uint32
	ProcessId          uint32 //nolint:revive,stylecheck // var-naming: struct field ProcessId should be ProcessID, but that's how the this field was named in windows
	WorkingSetSize     uint64
	VirtualSize        uint64
	UserModeTime       uint64
	ThreadCount        uint32
}

func (l *CheckProcess) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	timeZone, err := time.LoadLocation(l.timeZoneStr)
	if err != nil {
		return nil, fmt.Errorf("couldn't find timezone: %s", l.timeZoneStr)
	}

	processData := []winProcess{}
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
	err = wmi.QueryDefaultRetry(query, &processData)
	if err != nil {
		return nil, fmt.Errorf("wmi query failed: %s", err.Error())
	}
	runningProcs := map[string]winProcess{}

	// collect process state
	for i := range processData {
		process := processData[i]
		name := process.Name
		runningProcs[name] = process
	}

	if len(l.processes) == 0 || slices.Contains(l.processes, "*") {
		for i := range processData {
			process := processData[i]
			l.processes = append(l.processes, process.Name)
		}
	}

	for _, process := range l.processes {
		proc, exists := runningProcs[process]
		if !exists {
			continue
		}

		var state float64
		if proc.ProcessId == 0 || proc.ThreadCount == 0 {
			state = 0
		} else {
			state = 1
		}

		check.listData = append(check.listData, map[string]string{
			"process":          process,
			"state":            ProcessStates[strconv.FormatFloat(state, 'f', 0, 64)],
			"command_line":     proc.CommandLine,
			"creation":         proc.CreationDate.In(timeZone).Format("2006-01-02 15:04:05 MST"),
			"exe":              process,
			"filename":         proc.ExecutablePath,
			"handles":          fmt.Sprintf("%d", proc.HandleCount),
			"kernel":           fmt.Sprintf("%d", proc.KernelModeTime),
			"pagefile":         fmt.Sprintf("%d", proc.PageFileUsage),
			"peak_pagefile":    fmt.Sprintf("%d", proc.PeakPageFileUsage),
			"peak_virtual":     fmt.Sprintf("%d", proc.PeakVirtualSize),
			"peak_working_set": fmt.Sprintf("%d", proc.PeakWorkingSetSize),
			"pid":              fmt.Sprintf("%d", proc.ProcessId),
			"user":             fmt.Sprintf("%d", proc.UserModeTime),
			"virtual":          fmt.Sprintf("%d", proc.VirtualSize),
			"working_set":      fmt.Sprintf("%d", proc.WorkingSetSize),
		})
	}

	return check.Finalize()
}
