package snclient

import (
	"fmt"

	"pkg/utils"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/exp/slices"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func init() {
	AvailableChecks["check_service"] = CheckEntry{"check_service", new(CheckService)}
}

type CheckService struct{}

/* check_service_windows
 * Description: Checks the state of a windows service.
 */
func (l *CheckService) Check(_ *Agent, args []string) (*CheckResult, error) {
	services := []string{}
	excludes := []string{}
	check := &CheckData{
		result: &CheckResult{
			State: CheckExitOK,
		},
		conditionAlias: map[string]map[string]string{
			"state": {
				"started": "running",
			},
		},
		args: map[string]interface{}{
			"service": &services,
			"exclude": &excludes,
		},
		defaultFilter:   "none",
		defaultCritical: "state != 'running' && start_type = 'auto'",
		defaultWarning:  "state != 'running' && start_type = 'delayed'",
		detailSyntax:    "${name}=${state} (${start_type})",
		topSyntax:       "%(status): %(crit_list), delayed (%(warn_list))",
		okSyntax:        "%(status): All %(count) service(s) are ok.",
		emptySyntax:     "%(status): No services found",
		emptyState:      CheckExitUnknown,
	}
	_, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	// collect service state
	ctrlMgr, err := mgr.Connect()
	if err != nil {
		return &CheckResult{
			State:  int64(3),
			Output: fmt.Sprintf("Failed to open service handler: %s", err),
		}, nil
	}

	if len(services) == 0 || slices.Contains(services, "*") {
		serviceList, err := ctrlMgr.ListServices()
		if err != nil {
			return &CheckResult{
				State:  int64(3),
				Output: fmt.Sprintf("Failed to fetch service list: %s", err),
			}, nil
		}

		for _, service := range serviceList {
			if slices.Contains(excludes, service) {
				log.Tracef("service %s excluded by 'exclude' argument", service)

				continue
			}

			err = l.addService(check, ctrlMgr, service, services, excludes)
			if err != nil {
				return nil, err
			}
		}
	}

	// add services not yet added to the list
	for _, service := range services {
		if service == "*" {
			continue
		}
		found := false
		for _, e := range check.listData {
			if e["name"] == service || e["desc"] == service {
				found = true

				break
			}
		}
		if found {
			continue
		}

		err = l.addService(check, ctrlMgr, service, services, excludes)
		if err != nil {
			return nil, err
		}
	}

	return check.Finalize()
}

func (l *CheckService) getServiceDetails(ctrlMgr *mgr.Mgr, service string, servicesCount int) (*svc.Status, *mgr.Config, *process.MemoryInfoStat, *float64, error) {
	svcName, err := windows.UTF16PtrFromString(service)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to convert service name %s: %s", service, err.Error())
	}

	svcHdl, err := windows.OpenService(ctrlMgr.Handle, svcName, windows.SERVICE_QUERY_STATUS|windows.SERVICE_QUERY_CONFIG)
	if err != nil {
		if servicesCount == 0 {
			return nil, nil, nil, nil, nil
		}

		return nil, nil, nil, nil, fmt.Errorf("failed to open service %s: %s", service, err.Error())
	}
	ctlSvc := &mgr.Service{Name: service, Handle: svcHdl}
	defer ctlSvc.Close()

	statusCode, err := ctlSvc.Query()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to retrieve status code for service %s: %s", service, err.Error())
	}

	conf, err := ctlSvc.Config()
	if err != nil {
		log.Tracef("failed to retrieve service configuration for %s: %s", service, err.Error())
	}

	// retrieve process metrics
	var mem *process.MemoryInfoStat
	var cpu *float64
	if statusCode.State == windows.SERVICE_RUNNING {
		proc, _ := process.NewProcess(int32(statusCode.ProcessId))
		if proc != nil {
			mem, _ = proc.MemoryInfo()
			cpuP, err := proc.CPUPercent()
			if err == nil {
				cpu = &cpuP
			}
		}
	}

	return &statusCode, &conf, mem, cpu, nil
}

func (l *CheckService) addService(check *CheckData, ctrlMgr *mgr.Mgr, service string, services, excludes []string) error {
	statusCode, conf, mem, cpu, err := l.getServiceDetails(ctrlMgr, service, len(services))
	if err != nil {
		return err
	}
	if statusCode == nil {
		return nil
	}

	delayed := "0"
	if conf != nil && conf.DelayedAutoStart {
		delayed = "1"
	}

	listEntry := map[string]string{
		"name":           service,
		"service":        service,
		"state":          l.svcState(statusCode.State),
		"desc":           conf.DisplayName,
		"delayed":        delayed,
		"classification": l.svcClassification(conf.ServiceType),
		"pid":            fmt.Sprintf("%d", statusCode.ProcessId),
		"start_type":     l.svcStartType(conf.StartType, conf.DelayedAutoStart),
	}
	if mem != nil {
		listEntry["rss"] = fmt.Sprintf("%dB", mem.RSS)
		listEntry["vms"] = fmt.Sprintf("%dB", mem.VMS)
	}
	if cpu != nil {
		listEntry["cpu"] = fmt.Sprintf("%f%%", *cpu)
	}

	if !l.isRequired(check, listEntry, services, excludes) {
		return nil
	}

	check.listData = append(check.listData, listEntry)

	if len(services) == 0 && !check.showAll {
		return nil
	}

	l.addMetrics(check, service, statusCode, mem, cpu)

	return nil
}

func (l *CheckService) isRequired(check *CheckData, entry map[string]string, services, excludes []string) bool {
	name := entry["name"]
	desc := entry["desc"]
	if slices.Contains(excludes, name) || slices.Contains(excludes, desc) {
		log.Tracef("service %s excluded by exclude list", name)

		return false
	}
	if slices.Contains(services, "*") {
		return true
	}
	if len(services) > 0 && !slices.Contains(services, name) && !slices.Contains(services, desc) {
		log.Tracef("service %s excluded by not matching service list", name)

		return false
	}

	if !check.MatchMapCondition(check.filter, entry) {
		log.Tracef("service %s excluded by filter", name)

		return false
	}

	return true
}

func (l *CheckService) svcClassification(serviceType uint32) string {
	switch serviceType {
	case windows.SERVICE_KERNEL_DRIVER:
		return "kernel-driver"
	case windows.SERVICE_FILE_SYSTEM_DRIVER:
		return "system-driver"
	case windows.SERVICE_ADAPTER:
		return "service-adapter"
	case windows.SERVICE_RECOGNIZER_DRIVER:
		return "driver"
	case windows.SERVICE_WIN32_OWN_PROCESS:
		return "service-own-process"
	case windows.SERVICE_WIN32_SHARE_PROCESS:
		return "service-shared-process"
	case windows.SERVICE_WIN32:
		return "service"
	case windows.SERVICE_INTERACTIVE_PROCESS:
		return "interactive"
	}

	return "unknown"
}

func (l *CheckService) svcState(serviceState svc.State) string {
	switch serviceState {
	case windows.SERVICE_STOPPED:
		return "stopped"
	case windows.SERVICE_START_PENDING:
		return "starting"
	case windows.SERVICE_STOP_PENDING:
		return "stopping"
	case windows.SERVICE_RUNNING:
		return "running"
	case windows.SERVICE_CONTINUE_PENDING:
		return "continuing"
	case windows.SERVICE_PAUSE_PENDING:
		return "pausing"
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
		return "demand"
	case windows.SERVICE_DISABLED:
		return "disabled"
	}

	return "unknown"
}

func (l *CheckService) addMetrics(check *CheckData, service string, statusCode *svc.Status, mem *process.MemoryInfoStat, cpu *float64) {
	check.result.Metrics = append(check.result.Metrics, &CheckMetric{
		Name:  service,
		Value: float64(statusCode.State),
	})
	if mem != nil {
		check.result.Metrics = append(
			check.result.Metrics,
			&CheckMetric{
				Name:     fmt.Sprintf("%s rss", service),
				Value:    float64(mem.RSS),
				Unit:     "B",
				Warning:  check.TransformThreshold(check.warnThreshold, "rss", fmt.Sprintf("%s rss", service), "B", "B", 0),
				Critical: check.TransformThreshold(check.warnThreshold, "rss", fmt.Sprintf("%s rss", service), "B", "B", 0),
			},
			&CheckMetric{
				Name:     fmt.Sprintf("%s vms", service),
				Value:    float64(mem.VMS),
				Unit:     "B",
				Warning:  check.TransformThreshold(check.warnThreshold, "vms", fmt.Sprintf("%s vms", service), "B", "B", 0),
				Critical: check.TransformThreshold(check.warnThreshold, "vms", fmt.Sprintf("%s vms", service), "B", "B", 0),
			},
		)
	} else {
		check.result.Metrics = append(
			check.result.Metrics,
			&CheckMetric{
				Name:  fmt.Sprintf("%s rss", service),
				Value: "U",
			},
			&CheckMetric{
				Name:  fmt.Sprintf("%s vms", service),
				Value: "U",
			},
		)
	}
	if cpu != nil {
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			Name:     fmt.Sprintf("%s cpu", service),
			Value:    utils.ToPrecision(*cpu, 1),
			Unit:     "%",
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
		})
	} else {
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			Name:  fmt.Sprintf("%s cpu", service),
			Value: "U",
		})
	}
}
