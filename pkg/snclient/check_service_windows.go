package snclient

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/wmi"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func init() {
	AvailableChecks["check_service"] = CheckEntry{"check_service", NewCheckService}
}

type WindowsService struct {
	Name        string
	DisplayName string
}

type WindowsServiceDetails struct {
	Name   string
	Status *svc.Status
	Config *mgr.Config
}

type CheckService struct {
	AllServices []WindowsService
	services    []string
	excludes    []string
}

func NewCheckService() CheckHandler {
	return &CheckService{}
}

func (l *CheckService) Build() *CheckData {
	return &CheckData{
		name: "check_service",
		description: `Checks the state of one or multiple windows services.

There is a specific [check_service for linux](../check_service_linux) as well.`,
		implemented:  Windows,
		docTitle:     "service (windows)",
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		conditionAlias: map[string]map[string]string{
			"state": {
				"started": "running",
			},
		},
		args: map[string]CheckArgument{
			"service": {
				value:           &l.services,
				isFilter:        true,
				description:     "Name of the service to check (set to * to check all services). (case insensitive) Default: *",
				defaultWarning:  "state != 'running'",
				defaultCritical: "state != 'running'",
			},
			"exclude": {value: &l.excludes, description: "List of services to exclude from the check (mainly used when service is set to *) (case insensitive)"},
		},
		defaultFilter:   "none",
		defaultCritical: "state != 'running' && start_type = 'auto'",
		defaultWarning:  "state != 'running' && start_type = 'delayed'",
		detailSyntax:    "${name}=${state} (${start_type})",
		topSyntax:       "%(status) - %(crit_list), delayed (%(warn_list))",
		okSyntax:        "%(status) - All %(count) service(s) are ok.",
		emptySyntax:     "%(status) - No services found",
		emptyState:      CheckExitUnknown,
		attributes: []CheckAttribute{
			{name: "name", description: "The name of the service"},
			{name: "service", description: "Alias for name"},
			{name: "desc", description: "Description of the service"},
			{name: "state", description: "The state of the service, one of: stopped, starting, stopping, running, continuing, pausing, paused or unknown"},
			{name: "pid", description: "The pid of the service"},
			{name: "created", description: "Date when service was started", unit: UDate},
			{name: "age", description: "Seconds since service was started", unit: UDuration},
			{name: "rss", description: "Memory rss in bytes", unit: UByte},
			{name: "vms", description: "Memory vms in bytes", unit: UByte},
			{name: "cpu", description: "CPU usage in percent", unit: UPercent},
			{name: "delayed", description: "If the service is delayed, can be 0 or 1 "},
			{name: "classification", description: "Classification of the service, one of: " +
				"kernel-driver, system-driver, service-adapter, driver, service-own-process, service-shared-process, service or interactive"},
			{name: "start_type", description: "The configured start type, one of: boot, system, delayed, auto, demand, disabled or unknown"},
		},
		exampleDefault: `
Checking all services except some excluded ones:

    check_service exclude=edgeupdate exclude=RemoteRegistry
    OK - All 15 service(s) are ok |'count'=15;;;0 'failed'=0;;;0

Checking a single service:

    check_service service=dhcp
    OK - All 1 service(s) are ok.
	`,
		exampleArgs: "service=dhcp",
	}
}

// MgrConnectReadOnly establishes a read-only connection to the
// local service control manager.
func MgrConnectReadOnly() (*mgr.Mgr, error) {
	h, err := windows.OpenSCManager(nil, nil, windows.GENERIC_READ)
	if err != nil {
		return nil, fmt.Errorf("windows.OpenSCManager: %s", err.Error())
	}

	return &mgr.Mgr{Handle: h}, nil
}

func (l *CheckService) Check(ctx context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	// make excludes case insensitive
	for i := range l.excludes {
		l.excludes[i] = strings.ToLower(l.excludes[i])
	}

	// collect service state
	ctrlMgr, err := MgrConnectReadOnly()
	if err != nil {
		return &CheckResult{
			State:  int64(3),
			Output: fmt.Sprintf("Failed to open service handler: %s", err),
		}, nil
	}
	defer func() {
		LogDebug(ctrlMgr.Disconnect())
	}()

	if len(l.services) == 0 || slices.Contains(l.services, "*") {
		serviceList, err2 := ctrlMgr.ListServices()
		if err2 != nil {
			return &CheckResult{
				State:  int64(3),
				Output: fmt.Sprintf("Failed to fetch service list: %s", err2.Error()),
			}, nil
		}

		for _, service := range serviceList {
			if slices.Contains(l.excludes, strings.ToLower(strings.TrimSpace(service))) {
				log.Tracef("service %s excluded by 'exclude' argument", service)

				continue
			}

			err = l.addService(ctx, check, ctrlMgr, service, l.services, l.excludes)
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
			if strings.EqualFold(e["name"], service) || strings.EqualFold(e["desc"], service) {
				found = true

				break
			}
		}
		if found {
			continue
		}

		err = l.addService(ctx, check, ctrlMgr, service, l.services, l.excludes)
		if err != nil {
			return nil, err
		}
	}

	if len(l.services) == 0 && !check.showAll {
		check.addCountMetrics = true
		check.addProblemCountMetrics = true
	}

	return check.Finalize()
}

func (l *CheckService) getServiceDetails(ctrlMgr *mgr.Mgr, service string) (*WindowsServiceDetails, error) {
	realName, svcHdl, err := l.FindService(ctrlMgr, service)
	if err != nil {
		return nil, fmt.Errorf("failed to open service %s: %s", service, err.Error())
	}
	ctlSvc := &mgr.Service{Name: service, Handle: *svcHdl}
	defer func() {
		LogDebug(ctlSvc.Close())
	}()

	statusCode, err := ctlSvc.Query()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve status code for service %s: %s", service, err.Error())
	}

	conf, err := ctlSvc.Config()
	if err != nil {
		log.Tracef("failed to retrieve service configuration for %s: %s", service, err.Error())
	}

	return &WindowsServiceDetails{
		Name:   realName,
		Status: &statusCode,
		Config: &conf,
	}, nil
}

func (l *CheckService) addService(ctx context.Context, check *CheckData, ctrlMgr *mgr.Mgr, service string, services, excludes []string) error {
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

	// fetch memory / cpu for main process
	if details.Status.State == windows.SERVICE_RUNNING {
		err := l.addProcMetrics(ctx, listEntry["pid"], listEntry)
		if err != nil {
			log.Warnf("failed to add proc metrics: %s", err.Error())
		}
	}

	if !l.isRequired(check, listEntry, services, excludes) {
		return nil
	}

	check.listData = append(check.listData, listEntry)

	l.addServiceMetrics(service, float64(details.Status.State), check, listEntry)

	return nil
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
		if strings.EqualFold(s.DisplayName, name) {
			return s.Name, nil
		}
		if strings.EqualFold(s.Name, name) {
			return s.Name, nil
		}
	}

	return "", nil
}

func (l *CheckService) ListServicesWithDisplayname() ([]WindowsService, error) {
	services := []WindowsService{}
	err := wmi.QueryDefaultRetry("SELECT Name, DisplayName FROM Win32_Service", &services)
	if err != nil {
		return nil, fmt.Errorf("could not fetch service list: %s", err.Error())
	}

	return services, nil
}
