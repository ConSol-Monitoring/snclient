package snclient

import (
	"context"
	"fmt"

	"pkg/utils"
	"pkg/wmi"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/exp/slices"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func init() {
	AvailableChecks["check_service"] = CheckEntry{"check_service", new(CheckService)}
}

type WindowsService struct {
	Name        string
	DisplayName string
}

type WindowsServiceDetails struct {
	Name   string
	Status *svc.Status
	Config *mgr.Config
	Memory *process.MemoryInfoStat
	CPU    *float64
}

type CheckService struct {
	AllServices []WindowsService
	services    []string
	excludes    []string
}

func (l *CheckService) Build() *CheckData {
	l.services = []string{}
	l.excludes = []string{}

	return &CheckData{
		name:         "check_service",
		description:  "Checks the state of one or multiple windows services.",
		hasInventory: true,
		result: &CheckResult{
			State: CheckExitOK,
		},
		conditionAlias: map[string]map[string]string{
			"state": {
				"started": "running",
			},
		},
		args: map[string]interface{}{
			"service": &l.services,
			"exclude": &l.excludes,
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
}

func (l *CheckService) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	// collect service state
	ctrlMgr, err := mgr.Connect()
	if err != nil {
		return &CheckResult{
			State:  int64(3),
			Output: fmt.Sprintf("Failed to open service handler: %s", err),
		}, nil
	}

	if len(l.services) == 0 || slices.Contains(l.services, "*") {
		serviceList, err := ctrlMgr.ListServices()
		if err != nil {
			return &CheckResult{
				State:  int64(3),
				Output: fmt.Sprintf("Failed to fetch service list: %s", err),
			}, nil
		}

		for _, service := range serviceList {
			if slices.Contains(l.excludes, service) {
				log.Tracef("service %s excluded by 'exclude' argument", service)

				continue
			}

			err = l.addService(check, ctrlMgr, service, l.services, l.excludes)
			if err != nil {
				return nil, err
			}
		}
	}

	// add services not yet added to the list
	for _, service := range l.services {
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

		err = l.addService(check, ctrlMgr, service, l.services, l.excludes)
		if err != nil {
			return nil, err
		}
	}

	return check.Finalize()
}

func (l *CheckService) getServiceDetails(ctrlMgr *mgr.Mgr, service string) (*WindowsServiceDetails, error) {
	realName, svcHdl, err := l.FindService(ctrlMgr, service)
	if err != nil {
		return nil, fmt.Errorf("failed to open service %s: %s", service, err.Error())
	}
	ctlSvc := &mgr.Service{Name: service, Handle: *svcHdl}
	defer ctlSvc.Close()

	statusCode, err := ctlSvc.Query()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve status code for service %s: %s", service, err.Error())
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

	return &WindowsServiceDetails{
		Name:   realName,
		Status: &statusCode,
		Config: &conf,
		Memory: mem,
		CPU:    cpu,
	}, nil
}

func (l *CheckService) addService(check *CheckData, ctrlMgr *mgr.Mgr, service string, services, excludes []string) error {
	details, err := l.getServiceDetails(ctrlMgr, service)
	if err != nil {
		if len(services) == 0 {
			return nil
		}

		return err
	}

	delayed := "0"
	if details.Config.DelayedAutoStart {
		delayed = "1"
	}

	listEntry := map[string]string{
		"name":           service,
		"service":        details.Name,
		"state":          l.svcState(details.Status.State),
		"desc":           details.Config.DisplayName,
		"delayed":        delayed,
		"classification": l.svcClassification(details.Config.ServiceType),
		"pid":            fmt.Sprintf("%d", details.Status.ProcessId),
		"start_type":     l.svcStartType(details.Config.StartType, details.Config.DelayedAutoStart),
	}
	if details.Memory != nil {
		listEntry["rss"] = fmt.Sprintf("%dB", details.Memory.RSS)
		listEntry["vms"] = fmt.Sprintf("%dB", details.Memory.VMS)
	}
	if details.CPU != nil {
		listEntry["cpu"] = fmt.Sprintf("%f%%", *details.CPU)
	}

	if !l.isRequired(check, listEntry, services, excludes) {
		return nil
	}

	check.listData = append(check.listData, listEntry)

	if len(services) == 0 && !check.showAll {
		return nil
	}

	l.addMetrics(check, service, details.Status, details.Memory, details.CPU)

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

func (l *CheckService) FindService(ctrlMgr *mgr.Mgr, name string) (string, *windows.Handle, error) {
	svcName, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert service name %s: %s", name, err.Error())
	}

	svcHdl, sErr := windows.OpenService(ctrlMgr.Handle, svcName, windows.SERVICE_QUERY_STATUS|windows.SERVICE_QUERY_CONFIG)
	if sErr == nil {
		return name, &svcHdl, nil
	}

	// try display name
	if shortName, _ := l.GetNameByDisplayName(name); shortName != "" {
		svcName, err = windows.UTF16PtrFromString(shortName)
		if err != nil {
			return name, nil, fmt.Errorf("failed to convert service name %s: %s", name, err.Error())
		}

		svcHdl, err := windows.OpenService(ctrlMgr.Handle, svcName, windows.SERVICE_QUERY_STATUS|windows.SERVICE_QUERY_CONFIG)
		if err == nil {
			return shortName, &svcHdl, nil
		}
	}

	return name, nil, fmt.Errorf("failed to find service name %s: %s", name, sErr.Error())
}

// GetNameByDisplayName finds a service name from given displayname
func (l *CheckService) GetNameByDisplayName(name string) (string, error) {
	if l.AllServices == nil {
		serviceList, err := l.ListServicesWithDisplayname()
		if err != nil {
			return "", fmt.Errorf("failed to fetch service list: %s", err.Error())
		}
		l.AllServices = serviceList
	}

	for _, s := range l.AllServices {
		if s.DisplayName == name {
			return s.Name, nil
		}
	}

	return "", nil
}

func (l *CheckService) ListServicesWithDisplayname() ([]WindowsService, error) {
	querydata, err := wmi.Query("SELECT Name, DisplayName FROM Win32_Service")
	if err != nil {
		return nil, fmt.Errorf("could not fetch service list")
	}

	names := make([]WindowsService, 0)

	for _, row := range querydata {
		entry := WindowsService{}
		for _, v := range row {
			switch v.Key {
			case "Name":
				entry.Name = v.Value
			case "DisplayName":
				entry.DisplayName = v.Value
			}
		}
		names = append(names, entry)
	}

	return names, nil
}
