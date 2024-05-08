package snclient

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/wmi"
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
		emptyState:    ExitCodeUnknown,
		emptySyntax:   "query did not return any result row.",
		hasArgsFilter: true, // otherwise empty-syntax won't be applied
		topSyntax:     "${list}",
		detailSyntax:  "%(line)",
		exampleDefault: `
    check_wmi "query=select DeviceID, FreeSpace FROM Win32_LogicalDisk WHERE DeviceID = 'C:'"
    C:, 27955118080

Same query, but use output template for custom plugin output:

    check_wmi "query=select DeviceID, FreeSpace FROM Win32_LogicalDisk WHERE DeviceID = 'C:'" "detail-syntax= %(DeviceID) %(FreeSpace:h)"
    C: 27.94 G

Performance data will be extracted if the query contains at least 2 attributes. The first one must be a name, the others must be numeric.

    check_wmi query="select DeviceID, FreeSpace, Size FROM Win32_LogicalDisk"
	C:, 27199328256 |'C: FreeSpace'=27199328256

Use perf-config to format the performance data and apply thresholds:

    check_wmi query="select DeviceID, FreeSpace FROM Win32_LogicalDisk" perf-config="*(unit:B)" warn="FreeSpace > 1000000"
	C:, 27199328256 |'C: FreeSpace'=27199328256B;1000000
	`,
	}
}

func (l *CheckWMI) Check(_ context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("check_wmi is a windows only check")
	}
	enabled, _, _ := snc.config.Section("/modules").GetBool("CheckWMI")
	if !enabled {
		return nil, fmt.Errorf("module CheckWMI is not enabled in /modules section")
	}

	if l.query == "" {
		return nil, fmt.Errorf("wmi query required")
	}

	queryData, err := wmi.RawQuery(l.query)
	if err != nil {
		return nil, fmt.Errorf("wmi query failed: %s", err.Error())
	}

	for _, row := range queryData {
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

// AddPerfData function extracts labels and values from WMI query results and adds them as metrics.
// It uses the first column as name any the remaining columns as float64 value.
func (l *CheckWMI) AddPerfData(check *CheckData, row []wmi.Data) {
	if len(row) < 2 {
		log.Debugf("wmi query returned less than 2 columns. Extracting performance data requires at least 2.")

		return
	}

	prefix := row[0].Value

	for _, entry := range row[1:] {
		value, err := convert.Float64E(entry.Value)
		if err != nil {
			log.Debugf("value returned by wmi query cannot be converted to a float64. value='%s': %s", entry.Value, err.Error())

			continue
		}

		perfLabel := entry.Key
		if prefix != "" {
			perfLabel = fmt.Sprintf("%s %s", prefix, perfLabel)
		}

		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			ThresholdName: entry.Key,
			Name:          perfLabel,
			Value:         value,
			Warning:       check.warnThreshold,
			Critical:      check.critThreshold,
		})
	}
}
