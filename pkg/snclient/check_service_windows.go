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

type CheckService struct{}

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
func (l *CheckService) Check(_ *Agent, args []string) (*CheckResult, error) {
	check := &CheckData{
		result: &CheckResult{
			State: CheckExitOK,
		},
		detailSyntax: "%(service)=%(state)",
		topSyntax:    "%(crit_list), delayed (%(warn_list))",
		okSyntax:     "All %(count) service(s) are ok.",
	}
	argList, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	var services []string
	var statusCode svc.Status

	// parse threshold args
	for _, arg := range argList {
		if arg.key == "service" {
			services = append(services, arg.value)
		}
	}

	// collect service state
	ctrlMgr, err := mgr.Connect()
	if err != nil {
		return &CheckResult{
			State:  int64(3),
			Output: fmt.Sprintf("Failed to open service handler: %s", err),
		}, nil
	}

	serviceList, _ := ctrlMgr.ListServices()
	for _, service := range services {
		if slices.Contains(serviceList, service) {
			ctlSvc, err := ctrlMgr.OpenService(service)
			if err != nil {
				return &CheckResult{
					State:  int64(3),
					Output: fmt.Sprintf("Failed to open service %s: %s", service, err),
				}, nil
			}
			statusCode, _ = ctlSvc.Query()
			ctlSvc.Close()
		} else {
			return &CheckResult{
				State:  int64(3),
				Output: fmt.Sprintf("Service '%s' not found!", service),
			}, nil
		}

		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			Name:  service,
			Value: float64(statusCode.State),
		})

		check.listData = append(check.listData, map[string]string{
			"service": service,
			"state":   strconv.FormatInt(int64(statusCode.State), 10),
		})
	}

	check.Finalize()

	return check.result, nil
}
