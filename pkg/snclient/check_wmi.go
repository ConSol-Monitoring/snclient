package snclient

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
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
		l.AddPerfData(check, row)
	}

	return check.Finalize()
}

// AddPerfData extracts performance data from a WMI query result row and adds it to the check result.
// It expects a pointer to a CheckWMI object (l), a pointer to a CheckData object (check),
// and a slice of wmi.Data objects (row).
// If the row contains at least two elements, it attempts to parse the second element as a float64 value and adds it to the check result metrics.
// The first element is used as a label for the metric if available, with certain characters replaced for compatibility.
// If no label is provided, the key of the second element is used.
// The function ensures that the metric name is properly formatted and adds it along with the parsed value to the check result metrics.
// If parsing fails, it logs a message indicating the failure.
// If the row contains fewer than two elements, it logs a message indicating that the WMI query result is not formatted correctly for performance data.
func (l *CheckWMI) AddPerfData(check *CheckData, row []wmi.Data) {
	if len(row) >= 2 {
		value, err := strconv.ParseFloat(row[1].Value, 64)
		if err == nil {
			perfLabel := ""
			replacer := strings.NewReplacer(
				"-", "_",
				" ", "_",
				",", "_",
				".", "_",
				"/", "_",
				":", "_",
				"\\", "_",
				"__", "_",
				"___", "_",
			)
			if row[0].Value != "" {
				perfLabel = fmt.Sprintf("%s_%s", row[1].Key, replacer.Replace(row[0].Value))
			} else {
				perfLabel = replacer.Replace(row[1].Key)
			}
			check.result.Metrics = append(check.result.Metrics, &CheckMetric{
				Name:  strings.TrimRight(replacer.Replace(perfLabel), "_"),
				Value: value,
			})
		} else {
			log.Infof("value returned by wmi query cannot be converted to a float64. value=%s", row[1].Value)
		}
	} else {
		log.Infof("wmi query returned more than 2 columns. For perfdata we require only 2.")
	}
}
