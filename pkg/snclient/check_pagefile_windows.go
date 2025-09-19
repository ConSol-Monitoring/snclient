package snclient

import (
	"fmt"

	"github.com/consol-monitoring/snclient/pkg/humanize"
	"github.com/consol-monitoring/snclient/pkg/wmi"
)

type win32PageFile struct {
	Name              string
	CurrentUsage      uint64
	AllocatedBaseSize uint64
	PeakUsage         uint64
}

func (l *CheckPagefile) check(check *CheckData) (*CheckResult, error) {
	check.SetDefaultThresholdUnit("%", []string{"used", "used_pct", "free", "free_pct"})

	// https://learn.microsoft.com/en-us/windows/win32/cimwin32prov/win32-pagefileusage
	pageSizes := []win32PageFile{}
	err := wmi.QueryDefaultRetry("SELECT Name, CurrentUsage, AllocatedBaseSize, PeakUsage FROM Win32_PageFileUsage", &pageSizes)
	if err != nil {
		return nil, fmt.Errorf("wmi query failed: %s", err.Error())
	}

	totalData := map[string]uint64{}
	for _, pagefile := range pageSizes {
		pagefileData := map[string]uint64{}
		name := pagefile.Name

		value, _ := humanize.ParseBytes(fmt.Sprintf("%dMB", pagefile.AllocatedBaseSize))
		pagefileData["AllocatedBaseSize"] = value
		totalData["AllocatedBaseSize"] += value

		value, _ = humanize.ParseBytes(fmt.Sprintf("%dMB", pagefile.CurrentUsage))
		pagefileData["CurrentUsage"] = value
		totalData["CurrentUsage"] += value

		value, _ = humanize.ParseBytes(fmt.Sprintf("%dMB", pagefile.PeakUsage))
		pagefileData["PeakUsage"] = value
		totalData["PeakUsage"] += value

		l.addPagefile(check, name, pagefileData)
	}

	l.addPagefile(check, "total", totalData)

	return check.Finalize()
}

func (l *CheckPagefile) addPagefile(check *CheckData, name string, data map[string]uint64) {
	if data["AllocatedBaseSize"] == 0 {
		entry := map[string]string{
			"name":   name,
			"_error": "no data",
		}

		check.listData = append(check.listData, entry)

		return
	}

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

	// prevent adding metrics for filtered pagefiles
	if !check.MatchMapCondition(check.filter, entry, false) {
		log.Tracef("pagefile %s excluded by filter", name)

		return
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
