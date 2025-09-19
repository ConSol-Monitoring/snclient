package snclient

import (
	"context"
	"fmt"
	"runtime"
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
		detailSyntax:    "${name} ${used}/${size} (%(used_pct | fmt=%.1f )%)",
		topSyntax:       "%(status) - ${list}",
		attributes: []CheckAttribute{
			{name: "name", description: "The name of the page file (location)"},
			{name: "used", description: "Used memory in human readable bytes", unit: UByte},
			{name: "used_bytes", description: "Used memory in bytes", unit: UByte},
			{name: "used_pct", description: "Used memory in percent", unit: UPercent},
			{name: "free", description: "Free memory in human readable bytes", unit: UByte},
			{name: "free_bytes", description: "Free memory in bytes", unit: UByte},
			{name: "free_pct", description: "Free memory in percent", unit: UPercent},
			{name: "peak", description: "Peak memory usage in human readable bytes", unit: UByte},
			{name: "peak_bytes", description: "Peak memory in bytes", unit: UByte},
			{name: "peak_pct", description: "Peak memory in percent", unit: UPercent},
			{name: "size", description: "Total size of pagefile (human readable)", unit: UByte},
			{name: "size_bytes", description: "Total size of pagefile in bytes", unit: UByte},
		},
		exampleDefault: `
    check_pagefile
    OK - total 39.10 MiB/671.39 MiB (5.8%) |...
	`,
		exampleArgs: `'warn=used > 80%' 'crit=used > 95%'`,
	}
}

func (l *CheckPagefile) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	if runtime.GOOS != "windows" {
		// this allows to run make docs on Linux as well, even if it's not in the implemented: attribute
		return nil, fmt.Errorf("check_pagefile is a windows only check")
	}

	return l.check(check)
}
