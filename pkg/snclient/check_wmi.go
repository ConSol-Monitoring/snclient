package snclient

import (
	"context"
	"fmt"
	"strings"

	"pkg/wmi"
)

func init() {
	AvailableChecks["check_wmi"] = CheckEntry{"check_wmi", new(CheckWMI)}
}

type CheckWMI struct {
	query     string
	target    string
	user      string
	password  string
	namespace string
}

func (l *CheckWMI) Build() *CheckData {
	l.query = ""
	l.target = ""
	l.user = ""
	l.password = ""
	l.namespace = ""

	return &CheckData{
		name:        "check_wmi",
		description: "Check status and metrics by running wmi queries.",
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
	}
}

func (l *CheckWMI) Check(_ context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	enabled, _, _ := snc.Config.Section("/modules").GetBool("CheckWMI")
	if !enabled {
		return nil, fmt.Errorf("module CheckWMI is not enabled in /modules section")
	}

	if l.query == "" {
		return nil, fmt.Errorf("wmi query required")
	}

	querydata, err := wmi.Query(l.query)
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
