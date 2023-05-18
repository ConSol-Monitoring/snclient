package snclient

import (
	"fmt"

	"golang.org/x/exp/slices"
	"golang.org/x/sys/windows"
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
		defaultFilter:   "none",
		defaultCritical: "state = 'stopped' && start_type = 'auto'",
		defaultWarning:  "state = 'stopped' && start_type = 'delayed'",
		detailSyntax:    "${name}=${state} (${start_type})",
		topSyntax:       "%(status): %(crit_list), delayed (%(warn_list))",
		okSyntax:        "%(status): All %(count) service(s) are ok.",
		emptySyntax:     "%(status): No services found",
	}
	argList, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	services := []string{}
	excludes := []string{}

	// parse threshold args
	for _, arg := range argList {
		switch arg.key {
		case "service":
			services = append(services, arg.value)
		case "exclude":
			excludes = append(excludes, arg.value)
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

	serviceList, err := ctrlMgr.ListServices()
	if err != nil {
		return &CheckResult{
			State:  int64(3),
			Output: fmt.Sprintf("Failed to fetch service list: %s", err),
		}, nil
	}

	// add services from arguments
	for _, name := range services {
		if name != "*" {
			if !slices.Contains(serviceList, name) {
				serviceList = append(serviceList, name)
			}
		}
	}

	for _, service := range serviceList {
		if !check.MatchFilter("service", service) {
			log.Tracef("service %s excluded by filter", service)

			continue
		}
		if len(services) > 0 && !slices.Contains(services, service) && !slices.Contains(services, "*") {
			log.Tracef("service %s excluded by not matching service list", service)

			continue
		}
		if slices.Contains(excludes, service) {
			log.Tracef("service %s excluded by 'exclude' argument", service)

			continue
		}

		ctlSvc, err := ctrlMgr.OpenService(service)
		if err != nil {
			if len(services) == 0 {
				continue
			}

			return &CheckResult{
				State:  CheckExitUnknown,
				Output: fmt.Sprintf("Failed to open service %s: %s", service, err),
			}, nil
		}

		statusCode, err := ctlSvc.Query()
		if err != nil {
			return &CheckResult{
				State:  CheckExitUnknown,
				Output: fmt.Sprintf("Failed to retrieve status code for service %s: %s", service, err),
			}, nil
		}

		conf, err := ctlSvc.Config()
		if err != nil {
			if len(services) == 0 {
				continue
			}

			return &CheckResult{
				State:  CheckExitUnknown,
				Output: fmt.Sprintf("Failed to retrieve service configuration for %s: %s", service, err),
			}, nil
		}

		ctlSvc.Close()

		delayed := "0"
		if conf.DelayedAutoStart {
			delayed = "1"
		}
		check.listData = append(check.listData, map[string]string{
			"name":           service,
			"service":        service,
			"state":          l.svcState(statusCode.State),
			"desc":           conf.DisplayName,
			"delayed":        delayed,
			"classification": l.svcClassification(conf.ServiceType),
			"pid":            fmt.Sprintf("%d", statusCode.ProcessId),
			"start_type":     l.svcStartType(conf.StartType, conf.DelayedAutoStart),
		})

		if len(services) > 0 {
			check.result.Metrics = append(check.result.Metrics, &CheckMetric{
				Name:  service,
				Value: float64(statusCode.State),
			})
		}
	}

	return check.Finalize()
}

func (l *CheckService) svcClassification(serviceType uint32) string {
	switch serviceType {
	case windows.SERVICE_KERNEL_DRIVER:
		return "kernel-driver"
	case windows.SERVICE_FILE_SYSTEM_DRIVER:
		return "filesystem-driver"
	case windows.SERVICE_ADAPTER:
		return "service-adapater"
	case windows.SERVICE_RECOGNIZER_DRIVER:
		return "driver"
	case windows.SERVICE_WIN32_OWN_PROCESS:
		return "custom"
	case windows.SERVICE_WIN32_SHARE_PROCESS:
		return "shared-process"
	}

	return "unknown"
}

func (l *CheckService) svcState(serviceState svc.State) string {
	switch serviceState {
	case windows.SERVICE_STOPPED:
		return "stopped"
	case windows.SERVICE_START_PENDING:
		return "start-pending"
	case windows.SERVICE_STOP_PENDING:
		return "stop-pending"
	case windows.SERVICE_RUNNING:
		return "running"
	case windows.SERVICE_CONTINUE_PENDING:
		return "continue-pending"
	case windows.SERVICE_PAUSE_PENDING:
		return "pause-pending"
	case windows.SERVICE_PAUSED:
		return "paused"
	}

	return "unknown"
}

func (l *CheckService) svcStartType(startType uint32, delayed bool) string {
	switch startType {
	case windows.SERVICE_BOOT_START:
		return "boot"
	case windows.SERVICE_SYSTEM_START:
		return "system"
	case windows.SERVICE_AUTO_START:
		if delayed {
			return "delayed"
		}

		return "auto"
	case windows.SERVICE_DEMAND_START:
		return "manual"
	case windows.SERVICE_DISABLED:
		return "disabled"
	}

	return "unknown"
}
