package snclient

import (
	"fmt"
	"strconv"

	"internal/wmi"
)

func init() {
	AvailableChecks["check_wmi"] = CheckEntry{"check_wmi", new(CheckWMI)}
}

type CheckWMI struct {
	noCopy noCopy
	data   CheckData
}

/* check_wmi
 * Description: Querys the WMI for several metrics.
 * Thresholds: keys of the query
 * Units: none
 */
func (l *CheckWMI) Check(_ *Agent, args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(0)
	argList, err := ParseArgs(args, &l.data)
	if err != nil {
		return nil, fmt.Errorf("args error: %s", err.Error())
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
		return nil, fmt.Errorf("wmi query failed: %s", err.Error())
	}

	mdata := map[string]string{}
	perfMetrics := []*CheckMetric{}

	for _, d := range querydata[0] {
		mdata[d.Key] = d.Value
		if d.Key == l.data.warnThreshold.name || d.Key == l.data.critThreshold.name {
			value, _ := strconv.ParseFloat(d.Value, 64)
			perfMetrics = append(perfMetrics, &CheckMetric{
				Name:  d.Key,
				Value: value,
			})
		}
	}

	// compare metrics to thresholds
	if CompareMetrics(mdata, l.data.warnThreshold) {
		state = CheckExitWarning
	}

	if CompareMetrics(mdata, l.data.critThreshold) {
		state = CheckExitCritical
	}

	return &CheckResult{
		State:   state,
		Output:  output,
		Metrics: perfMetrics,
	}, nil
}
