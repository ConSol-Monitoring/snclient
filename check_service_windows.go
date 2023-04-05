package snclient

import (
	"fmt"
	"strconv"
	"strings"

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

var ServiceStates = map[string]string{
	"stopped":      "1",
	"dead":         "1",
	"startpending": "2",
	"stoppending":  "3",
	"running":      "4",
	"started":      "4",
}

/* check_service_windows
 * Description: Checks the state of a service on the host.
 * Tresholds: status
 * Units: stopped, dead, startpending, stoppedpending, running, started
 */
func (l *CheckService) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(0)
	var output string
	argList := ParseArgs(args)
	var warnTreshold Treshold
	var critTreshold Treshold
	var services []string
	var statusCode svc.Status
	detailSyntax := "%(service)=%(state)"
	topSyntax := "%(crit_list), delayed (%(warn_list))"
	okSyntax := "All %(count) service(s) are ok."
	var checkData map[string]string

	// parse treshold args
	for _, arg := range argList {
		switch arg.key {
		case "warn", "warning":
			warnTreshold = ParseTreshold(arg.value)
		case "crit", "critical":
			critTreshold = ParseTreshold(arg.value)
		case "service":
			services = append(services, arg.value)
		case "detail-syntax":
			detailSyntax = arg.value
		case "top-syntax":
			topSyntax = arg.value
		case "ok-syntax":
			okSyntax = arg.value
		}
	}

	metrics := make([]*CheckMetric, 0, len(services))
	okList := make([]string, 0, len(services))
	warnList := make([]string, 0, len(services))
	critList := make([]string, 0, len(services))

	warnTreshold.value = ServiceStates[warnTreshold.value]
	critTreshold.value = ServiceStates[critTreshold.value]

	// collect service state
	m, err := mgr.Connect()
	if err != nil {
		return &CheckResult{
			State:  int64(3),
			Output: fmt.Sprintf("Failed to open service handler: %s", err),
		}, nil
	}

	serviceList, _ := m.ListServices()

	for _, service := range services {

		if slices.Contains(serviceList, service) {

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

		metrics = append(metrics, &CheckMetric{Name: service, Value: float64(statusCode.State)})

		mdata := []MetricData{{name: "state", value: strconv.FormatInt(int64(statusCode.State), 10)}}
		sdata := map[string]string{
			"service": service,
			"state":   strconv.FormatInt(int64(statusCode.State), 10),
		}

		// compare ram metrics to tresholds
		if CompareMetrics(mdata, critTreshold) {
			critList = append(critList, ParseSyntax(detailSyntax, sdata))

			continue
		}

		if CompareMetrics(mdata, warnTreshold) {
			warnList = append(warnList, ParseSyntax(detailSyntax, sdata))

			continue
		}

		okList = append(okList, ParseSyntax(detailSyntax, sdata))
	}

	if len(critList) > 0 {
		state = CheckExitCritical
	} else if len(warnList) > 0 {
		state = CheckExitWarning
	}

	checkData = map[string]string{
		"status":    strconv.FormatInt(state, 10),
		"count":     strconv.FormatInt(int64(len(services)), 10),
		"ok_list":   strings.Join(okList, ", "),
		"warn_list": strings.Join(warnList, ", "),
		"crit_list": strings.Join(critList, ", "),
	}

	if state == CheckExitOK {
		output = ParseSyntax(okSyntax, checkData)
	} else {
		output = ParseSyntax(topSyntax, checkData)
	}

	return &CheckResult{
		State:   state,
		Output:  output,
		Metrics: metrics,
	}, nil
}
