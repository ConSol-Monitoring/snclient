package snclient

import (
	"context"
	"fmt"
	"runtime"

	"pkg/humanize"
	"pkg/wmi"
)

func init() {
	AvailableChecks["check_pagefile"] = CheckEntry{"check_pagefile", NewCheckPagefile}
}

type CheckPagefile struct{}

func NewCheckPagefile() CheckHandler {
	return &CheckPagefile{}
}

func (l *CheckPagefile) Build() *CheckData {
	return &CheckData{
		name:         "check_pagefile",
		description:  "Checks the pagefile usage.",
		implemented:  Windows,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		defaultFilter:   "name = 'total'",
		defaultWarning:  "used > 60%",
		defaultCritical: "used > 80%",
		detailSyntax:    "${name} ${used} (${size})",
		topSyntax:       "${status}: ${list}",
		attributes: []CheckAttribute{
			{name: "name", description: "The name of the page file (location)"},
			{name: "used", description: "Used memory in human readable bytes"},
			{name: "used_bytes", description: "Used memory in bytes"},
			{name: "used_pct", description: "Used memory in percent"},
			{name: "free", description: "Free memory in human readable bytes"},
			{name: "free_bytes", description: "Free memory in bytes"},
			{name: "free_pct", description: "Free memory in percent"},
			{name: "peak", description: "Peak memory usage in human readable bytes"},
			{name: "peak_bytes", description: "Peak memory in bytes"},
			{name: "peak_pct", description: "Peak memory in percent"},
			{name: "size", description: "Total size of pagefile (human readable)"},
			{name: "size_bytes", description: "Total size of pagefile in bytes"},
		},
		exampleDefault: `
    check_pagefile
    OK: total 53.41 MiB (244.14 MiB) |...
	`,
		exampleArgs: `'warn=used > 80%' 'crit=used > 95%'`,
	}
}

func (l *CheckPagefile) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("check_pagefile is a windows only check")
	}
	check.SetDefaultThresholdUnit("%", []string{"used", "free"})
	check.ExpandThresholdUnit([]string{"k", "m", "g", "p", "e", "ki", "mi", "gi", "pi", "ei"}, "B", []string{"used", "free"})

	querydata, err := wmi.Query("SELECT Name, CurrentUsage, AllocatedBaseSize, PeakUsage FROM Win32_PageFileUsage")
	if err != nil {
		return nil, fmt.Errorf("wmi query failed: %s", err.Error())
	}

	totalData := map[string]uint64{"CurrentUsage": 0, "PeakUsage": 0, "AllocatedBaseSize": 0}
	for _, pagefile := range querydata {
		pagefileData := map[string]uint64{}
		var name string
		for _, data := range pagefile {
			if data.Key == "Name" {
				name = data.Value

				continue
			}
			value, _ := humanize.ParseBytes(data.Value + "MB")
			pagefileData[data.Key] = value
			totalData[data.Key] += value
		}
		l.addPagefile(check, name, pagefileData)
	}

	l.addPagefile(check, "total", totalData)

	return check.Finalize()
}

func (l *CheckPagefile) addPagefile(check *CheckData, name string, data map[string]uint64) {
	entry := map[string]string{
		"name":       name,
		"used":       humanize.IBytesF(data["CurrentUsage"], 2),
		"used_bytes": fmt.Sprintf("%d", data["CurrentUsage"]),
		"used_pct":   fmt.Sprintf("%.3f", (float64(data["CurrentUsage"]) * 100 / (float64(data["AllocatedBaseSize"])))),
		"free":       humanize.IBytesF((data["AllocatedBaseSize"]-data["CurrentUsage"])*100/data["AllocatedBaseSize"], 2),
		"free_bytes": fmt.Sprintf("%d", data["AllocatedBaseSize"]-data["CurrentUsage"]),
		"free_pct":   fmt.Sprintf("%.3f", (float64(data["AllocatedBaseSize"]-data["CurrentUsage"]) * 100 / (float64(data["AllocatedBaseSize"])))),
		"peak":       humanize.IBytesF(data["PeakUsage"], 2),
		"peak_bytes": fmt.Sprintf("%d", data["PeakUsage"]),
		"peak_pct":   fmt.Sprintf("%.3f", (float64(data["PeakUsage"]) * 100 / (float64(data["AllocatedBaseSize"])))),
		"size":       humanize.IBytesF(data["AllocatedBaseSize"], 2),
		"size_bytes": fmt.Sprintf("%d", data["AllocatedBaseSize"]),
	}
	check.listData = append(check.listData, entry)
	if check.HasThreshold("free") {
		check.AddBytePercentMetrics("free", name, float64(data["AllocatedBaseSize"]-data["CurrentUsage"]), float64(data["AllocatedBaseSize"]))
	}
	if check.HasThreshold("used") {
		check.AddBytePercentMetrics("used", name, float64(data["CurrentUsage"]), float64(data["AllocatedBaseSize"]))
	}
	if check.HasThreshold("peak") {
		check.AddBytePercentMetrics("peak", name, float64(data["PeakUsage"]), float64(data["AllocatedBaseSize"]))
	}
	if check.HasThreshold("free_pct") {
		check.AddPercentMetrics("free_pct", name, float64(data["AllocatedBaseSize"]-data["CurrentUsage"]), float64(data["AllocatedBaseSize"]))
	}
	if check.HasThreshold("used_pct") {
		check.AddPercentMetrics("used_pct", name, float64(data["AllocatedBaseSize"]-data["CurrentUsage"]), float64(data["AllocatedBaseSize"]))
	}
	if check.HasThreshold("peak_pct") {
		check.AddPercentMetrics("used_pct", name, float64(data["AllocatedBaseSize"]-data["PeakUsage"]), float64(data["AllocatedBaseSize"]))
	}
}
