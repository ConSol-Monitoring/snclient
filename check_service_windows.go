package snclient

import (
	"fmt"
	"strconv"

	"golang.org/x/exp/slices"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func init() {
	AvailableChecks["check_service"] = CheckEntry{"check_service", new(CheckService)}
}

type CheckService struct {
	noCopy noCopy
}

var SERVICE_STATE = map[string]string{
	"stopped":      "1",
	"startpending": "2",
	"stoppending":  "3",
	"running":      "4",
	"started":      "4",
}

/* check_service todo
 * todo
 */
func (l *CheckService) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(0)
	output := "Service is ok."
	argList := ParseArgs(args)
	var warnTreshold Treshold
	var critTreshold Treshold
	var service string
	var statusCode svc.Status

	// parse treshold args
	for _, arg := range argList {
		switch arg.key {
		case "warn", "warning":
			warnTreshold = ParseTreshold(arg.value)
		case "crit", "critical":
			critTreshold = ParseTreshold(arg.value)
		case "service":
			service = arg.value
		}
	}

	// collect service state
	m, err := mgr.Connect()
	if err != nil {
		return &CheckResult{
			State:  int64(3),
			Output: fmt.Sprintf("Failed to open service handler: %s", err),
		}, nil
	}

	services, _ := m.ListServices()
	if slices.Contains(services, service) {

		s, err := m.OpenService(service)
		if err != nil {
			return &CheckResult{
				State:  int64(3),
				Output: fmt.Sprintf("Failed to open service %s: %s", service, err),
			}, nil
		}
		defer s.Close()

		statusCode, _ = s.Query()

	} else {
		return &CheckResult{
			State:  int64(3),
			Output: fmt.Sprintf("Service '%s' not found!", service),
		}, nil
	}

	warnTreshold.value = SERVICE_STATE[warnTreshold.value]
	critTreshold.value = SERVICE_STATE[critTreshold.value]

	mdata := []MetricData{{name: "state", value: strconv.FormatInt(int64(statusCode.State), 10)}}

	// compare ram metrics to tresholds
	if CompareMetrics(mdata, warnTreshold) {
		state = CheckExitWarning
		output = "Service is warning!"
	}

	if CompareMetrics(mdata, critTreshold) {
		state = CheckExitCritical
		output = "Service is critical!"
	}

	return &CheckResult{
		State:  state,
		Output: output,
		Metrics: []*CheckMetric{
			{
				Name:  service,
				Value: float64(statusCode.State),
			},
		},
	}, nil
}
