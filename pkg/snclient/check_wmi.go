package snclient

import (
	"fmt"
	"strings"

	"pkg/wmi"
)

func init() {
	AvailableChecks["check_wmi"] = CheckEntry{"check_wmi", new(CheckWMI)}
}

type CheckWMI struct{}

func (l *CheckWMI) Check(snc *Agent, args []string) (*CheckResult, error) {
	query := ""
	target := ""
	user := ""
	password := ""
	namespace := ""
	check := &CheckData{
		name:        "check_wmi",
		description: "Check status and metrics by running wmi queries.",
		args: map[string]interface{}{
			"query":     &query,
			"target":    &target,
			"namespace": &namespace,
			"user":      &user,
			"password":  &password,
		},
		result: &CheckResult{
			State: CheckExitOK,
		},
		topSyntax:    "${list}",
		detailSyntax: "%(line)",
	}
	_, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

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

	if query == "" {
		return nil, fmt.Errorf("wmi query required")
	}

	querydata, err := wmi.Query(query)
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
