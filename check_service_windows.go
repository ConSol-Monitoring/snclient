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
	data   CheckData
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
 * Thresholds: status
 * Units: stopped, dead, startpending, stoppedpending, running, started
 */
func (l *CheckService) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(0)
	var output string
	l.data.detailSyntax = "%(service)=%(state)"
	l.data.topSyntax = "%(crit_list), delayed (%(warn_list))"
	l.data.okSyntax = "All %(count) service(s) are ok."
	argList, err := ParseArgs(args, &l.data)
	if err != nil {
		return nil, fmt.Errorf("args error: %s", err.Error())
	}
	var services []string
	var statusCode svc.Status
	var checkData map[string]string

	// parse threshold args
	for _, arg := range argList {
		if arg.key == "service" {
			services = append(services, arg.value)
		}
	}

	metrics := make([]*CheckMetric, 0, len(services))
	okList := make([]string, 0, len(services))
	warnList := make([]string, 0, len(services))
	critList := make([]string, 0, len(services))

	l.data.warnThreshold.value = ServiceStates[l.data.warnThreshold.value]
	l.data.critThreshold.value = ServiceStates[l.data.critThreshold.value]

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

		mdata := map[string]string{
			"service": service,
			"state":   strconv.FormatInt(int64(statusCode.State), 10),
		}

		// compare ram metrics to thresholds
		if CompareMetrics(mdata, l.data.critThreshold) {
			critList = append(critList, ParseSyntax(l.data.detailSyntax, mdata))

			continue
		}

		if CompareMetrics(mdata, l.data.warnThreshold) {
			warnList = append(warnList, ParseSyntax(l.data.detailSyntax, mdata))

			continue
		}

		okList = append(okList, ParseSyntax(l.data.detailSyntax, mdata))
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
		output = ParseSyntax(l.data.okSyntax, checkData)
	} else {
		output = ParseSyntax(l.data.topSyntax, checkData)
	}

	return &CheckResult{
		State:   state,
		Output:  output,
		Metrics: metrics,
	}, nil
}
