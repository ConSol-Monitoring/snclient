package snclient

import (
	"fmt"
	"strconv"

	"pkg/wmi"
)

func init() {
	AvailableChecks["check_wmi"] = CheckEntry{"check_wmi", new(CheckWMI)}
}

type CheckWMI struct{}

/* check_wmi
 * Description: Querys the WMI for several metrics.
 * Thresholds: keys of the query
 * Units: none
 */
func (l *CheckWMI) Check(_ *Agent, args []string) (*CheckResult, error) {
	check := &CheckData{
		result: &CheckResult{
			State: CheckExitOK,
		},
	}
	argList, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	var query string

	// parse threshold args
	for _, arg := range argList {
		switch arg.key {
		case "query":
			query = arg.value
		default:
			log.Debugf("unknown argument: %s", arg.key)
		}
	}

	// query wmi
	querydata, output, err := wmi.Query(query)
	if err != nil {
		return nil, fmt.Errorf("wmi query failed: %s%s", output, err.Error())
	}

	for _, d := range querydata[0] {
		value, _ := strconv.ParseFloat(d.Value, 64)
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			Name:  d.Key,
			Value: value,
		})
	}

	return check.Finalize()
}
