package snclient

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"pkg/wmi"
)

func init() {
	AvailableChecks["check_wmi"] = CheckEntry{"check_wmi", NewCheckWMI}
}

type CheckWMI struct {
	query     string
	target    string
	user      string
	password  string
	namespace string
}

func NewCheckWMI() CheckHandler {
	return &CheckWMI{}
}

func (l *CheckWMI) Build() *CheckData {
	return &CheckData{
		name:        "check_wmi",
		description: "Check status and metrics by running wmi queries.",
		implemented: Windows,
		args: map[string]CheckArgument{
			"query":     {value: &l.query, description: "The WMI query to execute"},
			"target":    {value: &l.target, description: "Unused and not supported for now"},
			"namespace": {value: &l.namespace, description: "Unused and not supported for now"},
			"user":      {value: &l.user, description: "Unused and not supported for now"},
			"password":  {value: &l.password, description: "Unused and not supported for now"},
		},
		result: &CheckResult{
			State: CheckExitOK,
		},
		topSyntax:    "${list}",
		detailSyntax: "%(line)",
		exampleDefault: `
    check_wmi "query=select FreeSpace, DeviceID FROM Win32_LogicalDisk WHERE DeviceID = 'C:'"
    27955118080, C:

Same query, but use output template for custom plugin output:

    check_wmi "query=select FreeSpace, DeviceID FROM Win32_LogicalDisk WHERE DeviceID = 'C:'" "detail-syntax= %(DeviceID) %(FreeSpace:h)"
    C: 27.94 G
	`,
	}
}

func (l *CheckWMI) Check(_ context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("check_wmi is a windows only check")
	}
	enabled, _, _ := snc.Config.Section("/modules").GetBool("CheckWMI")
	if !enabled {
		return nil, fmt.Errorf("module CheckWMI is not enabled in /modules section")
	}

	if l.query == "" {
		return nil, fmt.Errorf("wmi query required")
	}

	querydata, err := wmi.RawQuery(l.query)
	if err != nil {
		return nil, fmt.Errorf("wmi query failed: %s", err.Error())
	}

	for _, row := range querydata {
		values := []string{}
		entry := map[string]string{}
		for k := range row {
			entry[row[k].Key] = row[k].Value
			values = append(values, row[k].Value)
		}
		check.listData = append(check.listData, entry)
		entry["line"] = strings.Join(values, ", ")
	}

	return check.Finalize()
}
