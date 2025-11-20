package snclient

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/consol-monitoring/snclient/pkg/wmi"
)

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

func (l *CheckProcess) fetchProcs(_ context.Context, check *CheckData) error {
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
	err := wmi.QueryDefaultRetry(query, &processData)
	if err != nil {
		return fmt.Errorf("wmi query failed: %s", err.Error())
	}

	for i := range processData {
		proc := processData[i]

		if len(l.processes) > 0 && !slices.Contains(l.processes, strings.ToLower(proc.Name)) && !slices.Contains(l.processes, "*") {
			continue
		}

		state := "stopped"
		if proc.ProcessId > 0 && proc.ThreadCount > 0 {
			state = "started"
		}

		cpu := float64(0)
		cpuSec := float64(proc.UserModeTime+proc.KernelModeTime) / 1e7 // values are multiple of 100ns
		age := time.Since(proc.CreationDate).Seconds()
		if age > 0 {
			cpu = (cpuSec / age) * 100
		}

		check.listData = append(check.listData, map[string]string{
			"process":          proc.Name,
			"state":            state,
			"command_line":     proc.CommandLine,
			"creation":         fmt.Sprintf("%d", proc.CreationDate.Unix()),
			"exe":              proc.Name,
			"filename":         proc.ExecutablePath,
			"handles":          fmt.Sprintf("%d", proc.HandleCount),
			"kernel":           fmt.Sprintf("%f", float64(proc.KernelModeTime)/1e7), // values are multiple of 100ns
			"pagefile":         fmt.Sprintf("%d", proc.PageFileUsage),
			"peak_pagefile":    fmt.Sprintf("%d", proc.PeakPageFileUsage),
			"peak_virtual":     fmt.Sprintf("%d", proc.PeakVirtualSize),
			"peak_working_set": fmt.Sprintf("%d", proc.PeakWorkingSetSize),
			"pid":              fmt.Sprintf("%d", proc.ProcessId),
			"user":             fmt.Sprintf("%f", float64(proc.UserModeTime)/1e7), // values are multiple of 100ns
			"virtual":          fmt.Sprintf("%d", proc.VirtualSize),
			"working_set":      fmt.Sprintf("%d", proc.WorkingSetSize),
			"rss":              fmt.Sprintf("%d", proc.WorkingSetSize),
			"cpu":              fmt.Sprintf("%f", cpu),
			"cpu_seconds":      fmt.Sprintf("%f", cpuSec),
		})
	}

	return nil
}
