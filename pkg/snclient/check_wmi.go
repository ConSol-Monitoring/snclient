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
		args: map[string]interface{}{
			"query":     &l.query,
			"target":    &l.target,
			"namespace": &l.namespace,
			"user":      &l.user,
			"password":  &l.password,
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

	for _, k := range []string{"target", "user", "password", "namespace"} {
		if check.args[k] != nil {
			if str, ok := check.args[k].(*string); !ok || *str != "" {
				return nil, fmt.Errorf("CheckWMI: '%s' attribute is not supported", k)
			}
		}
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
