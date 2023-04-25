package snclient

import (
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
 * Tresholds: keys of the query
 * Units: none
 */
func (l *CheckWMI) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(0)
	argList := ParseArgs(args, &l.data)
	var query string

	// parse treshold args
	for _, arg := range argList {
		if arg.key == "query" {
			query = arg.value
		default:
			log.Debugf("unknown argument: %s", arg.key)
		}
	}

	// query wmi
	querydata, output := wmi.Query(query)

	mdata := map[string]string{}
	perfMetrics := []*CheckMetric{}

	for _, d := range querydata[0] {
		mdata[d.Key] = d.Value
		if d.Key == l.data.warnTreshold.name || d.Key == l.data.critTreshold.name {
			value, _ := strconv.ParseFloat(d.Value, 64)
			perfMetrics = append(perfMetrics, &CheckMetric{
				Name:  d.Key,
				Value: value,
			})
		}
	}

	// compare metrics to tresholds
	if CompareMetrics(mdata, l.data.warnTreshold) {
		state = CheckExitWarning
	}

	if CompareMetrics(mdata, l.data.critTreshold) {
		state = CheckExitCritical
	}

	return &CheckResult{
		State:   state,
		Output:  output,
		Metrics: perfMetrics,
	}, nil
}
