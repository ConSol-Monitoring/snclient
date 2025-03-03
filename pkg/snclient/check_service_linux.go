package snclient

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

func init() {
	AvailableChecks["check_service"] = CheckEntry{"check_service", NewCheckService}
}

var (
	reSvcDetails   = regexp.MustCompile(`(?s)^.*?\.service(?:\s\-\s(.*?)|)\n.*Active:\s*([A-Za-z() :-]+)(?:\ssince|\n)`)
	reSvcMainPid   = regexp.MustCompile(`Main\sPID:\s(\d+)`)
	reSvcPidMaster = regexp.MustCompile(`─(\d+).+\(master\)`)
	reSvcPidCgroup = regexp.MustCompile(`[├─└]─\s*(\d+)\s+`)
	reSvcPreset    = regexp.MustCompile(`\s+preset:\s+(\w+)\)`)
	reSvcTasks     = regexp.MustCompile(`Tasks:\s*(\d+)`)
	reSvcStatic    = regexp.MustCompile(`;\sstatic\)`)
	reSvcActive    = regexp.MustCompile(`\s*Active:\s+(\S+)`)
	reSvcFirstLine = regexp.MustCompile(`^.\s(\S+)\.service\s+`)
	reSvcNameLine  = regexp.MustCompile(`^\s*(\S+)\.service\s+`)
)

const (
	systemctlStatusCmd = "systemctl status --lines=0 --no-pager --quiet"
	systemctlNames     = "systemctl list-units --lines=0 --no-pager --quiet --no-legend"
)

type CheckService struct {
	snc      *Agent
	services []string
	excludes []string
}

func NewCheckService() CheckHandler {
	return &CheckService{}
}

func (l *CheckService) Build() *CheckData {
	stateCondition := "( state not in ('running', 'oneshot', 'static') || active = 'failed' ) "

	return &CheckData{
		name: "check_service",
		description: `Checks the state of one or multiple linux (systemctl) services.

There is a specific [check_service for windows](../check_service_windows) as well.`,
		implemented:  Linux,
		docTitle:     "service (linux)",
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
				defaultCritical: stateCondition,
			},
			"exclude": {value: &l.excludes, description: "List of services to exclude from the check (mainly used when service is set to *) (case insensitive)"},
		},
		defaultFilter:   "active != inactive",
		defaultCritical: stateCondition + " && preset != 'disabled'",
		detailSyntax:    "${name}=${state}",
		topSyntax:       "%(status) - %(crit_list)",
		okSyntax:        "%(status) - All %(count) service(s) are ok.",
		emptySyntax:     "%(status) - No services found",
		emptyState:      CheckExitUnknown,
		attributes: []CheckAttribute{
			{name: "name", description: "The name of the service"},
			{name: "service", description: "Alias for name"},
			{name: "desc", description: "Description of the service"},
			{name: "active", description: "The active attribute of a service, one of: active, inactive or failed"},
			{name: "state", description: "The state of the service, one of: stopped, starting, oneshot, running, static or unknown"},
			{name: "pid", description: "The pid of the service"},
			{name: "created", description: "Date when service was started (unix timestamp)"},
			{name: "age", description: "Seconds since service was started"},
			{name: "rss", description: "Memory rss in bytes (main process)"},
			{name: "vms", description: "Memory vms in bytes (main process)"},
			{name: "cpu", description: "CPU usage in percent (main process)"},
			{name: "preset", description: "The preset attribute of the service, one of: enabled or disabled"},
			{name: "tasks", description: "Number of tasks for this service"},
		},
		exampleDefault: `
Checking all services except some excluded ones:

    check_service exclude=bluetooth
    OK - All 74 service(s) are ok.

Or check a specific service and get some metrics:

    check_service service=docker ok-syntax='${top-syntax}' top-syntax='%(status) - %(list)' detail-syntax='%(name) %(state) - memory: %(rss:h)B - age: %(age:duration)'
    OK - docker running - memory: 805.2MB - created: Fri 2023-11-17 20:34:01 CET |'docker'=4 'docker rss'=805200000B

Check memory usage of specific service:

    check_service service=docker warn='rss > 1GB' warn='rss > 2GB'
    OK - All 1 service(s) are ok. |'docker'=4 'docker rss'=59691008B;;;0 'docker vms'=3166244864B;;;0 'docker cpu'=0.7%;;;0 'docker tasks'=20;;;0
	`,
		exampleArgs: "service=docker",
	}
}

func (l *CheckService) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	l.snc = snc

	// make excludes case insensitive
	for i := range l.excludes {
		l.excludes[i] = strings.ToLower(l.excludes[i])
	}

	if len(l.services) == 0 || slices.Contains(l.services, "*") {
		// fetch status of all services at once instead of calling systemctl over and over
		output, stderr, _, err := snc.execCommand(ctx, fmt.Sprintf("%s --type=service --all", systemctlStatusCmd), DefaultCmdTimeout)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch service list: %s%s", err.Error(), stderr)
		}

		err = l.parseAllServices(ctx, check, output)
		if err != nil {
			return nil, err
		}
	}

	// add user supplied services not yet added
	for _, service := range l.services {
		if service == "*" {
			continue
		}
		found := false
		for _, e := range check.listData {
			if strings.EqualFold(e["name"], service) {
				found = true

				break
			}
		}
		if found {
			continue
		}

		err := l.addServiceByName(ctx, check, service, l.services, l.excludes)
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

func (l *CheckService) addServiceByName(ctx context.Context, check *CheckData, service string, services, excludes []string) error {
	err := l.addServiceByExactName(ctx, check, service, services, excludes)
	if err == nil {
		return nil
	}

	realService := l.findServiceByName(ctx, service)
	if realService == "" {
		return err
	}

	return l.addServiceByExactName(ctx, check, realService, services, excludes)
}

func (l *CheckService) addServiceByExactName(ctx context.Context, check *CheckData, service string, services, excludes []string) error {
	output, stderr, _, err := l.snc.execCommand(ctx, fmt.Sprintf("%s %s.service ", systemctlStatusCmd, service), DefaultCmdTimeout)
	if err != nil {
		return fmt.Errorf("systemctl failed: %s\n%s", err.Error(), stderr)
	}

	if match, _ := regexp.MatchString(`Unit .* could not be found`, stderr); match || len(output) < 1 {
		return fmt.Errorf("could not find service: %s", service)
	}

	listEntry := l.parseSystemCtlStatus(service, output)

	return l.addService(ctx, check, service, listEntry, services, excludes)
}

func (l *CheckService) addService(ctx context.Context, check *CheckData, service string, listEntry map[string]string, services, excludes []string) error {
	// fetch memory / cpu for main process
	if listEntry["state"] == "running" {
		err := l.addProcMetrics(ctx, listEntry["pid"], listEntry)
		if err != nil {
			log.Warnf("failed to add proc metrics: %s", err.Error())
		}
	}

	if !l.isRequired(check, listEntry, services, excludes) {
		return nil
	}

	check.listData = append(check.listData, listEntry)

	// do not add metrics for all services unless requested
	if len(services) == 0 && !check.showAll {
		return nil
	}

	l.addServiceMetrics(service, l.svcStateFloat(listEntry["state"]), check, listEntry)

	return nil
}

func (l *CheckService) svcState(serviceState string) string {
	switch serviceState {
	case "inactive (dead)":
		return "stopped"
	case "activating (start)":
		return "starting"
	case "active (exited)":
		return "oneshot"
	case "active (running)":
		return "running"
	}
	if strings.HasPrefix(serviceState, "failed") {
		return "stopped"
	}

	return "unknown"
}

func (l *CheckService) svcStateFloat(serviceState string) float64 {
	switch serviceState {
	case "stopped":
		return float64(1)
	case "starting":
		return float64(2)
	case "oneshot":
		return float64(3)
	case "running":
		return float64(4)
	case "static":
		return float64(5)
	}

	return float64(0)
}

func (l *CheckService) parseSystemCtlStatus(name, output string) (listEntry map[string]string) {
	listEntry = map[string]string{
		"name":    name,
		"service": name,
		"active":  "",
		"state":   "unknown",
		"created": "",
		"age":     "",
		"preset":  "",
		"desc":    "",
		"pid":     "",
		"rss":     "",
		"vms":     "",
		"cpu":     "",
		"tasks":   "",
	}

	match := reSvcDetails.FindStringSubmatch(output)
	if len(match) > 2 {
		listEntry["desc"] = match[1]
		listEntry["state"] = l.svcState(match[2])
	}

	match = reSvcMainPid.FindStringSubmatch(output)
	if len(match) >= 1 {
		listEntry["pid"] = match[1]
	}
	if listEntry["pid"] == "" {
		match = reSvcPidMaster.FindStringSubmatch(output)
		if len(match) >= 1 {
			listEntry["pid"] = match[1]
		}
	}
	if listEntry["pid"] == "" {
		matches := reSvcPidCgroup.FindAllStringSubmatch(output, -1)
		pids := []string{}
		for _, m := range matches {
			pids = append(pids, m[1])
		}
		listEntry["pid"] = strings.Join(pids, ",")
	}

	match = reSvcTasks.FindStringSubmatch(output)
	if len(match) > 1 {
		listEntry["tasks"] = match[1]
	}

	match = reSvcPreset.FindStringSubmatch(output)
	if len(match) > 1 {
		listEntry["preset"] = match[1]
	}

	match = reSvcStatic.FindStringSubmatch(output)
	if len(match) > 0 {
		listEntry["state"] = "static"
	}

	match = reSvcActive.FindStringSubmatch(output)
	if len(match) > 1 {
		listEntry["active"] = match[1]
	}

	return listEntry
}

func (l *CheckService) parseAllServices(ctx context.Context, check *CheckData, output string) (err error) {
	// services are separated by two empty lines
	services := strings.Split(output, "\n\n")
	for _, svc := range services {
		serviceMatches := reSvcFirstLine.FindStringSubmatch(svc)
		if len(serviceMatches) < 2 {
			log.Tracef("no service name found in systemctl output:\n%s", svc)

			continue
		}
		service := serviceMatches[1]

		if slices.Contains(l.excludes, strings.ToLower(service)) {
			log.Tracef("service %s excluded by 'exclude' argument", service)

			continue
		}

		listEntry := l.parseSystemCtlStatus(service, svc)
		err = l.addService(ctx, check, service, listEntry, l.services, l.excludes)
		if err != nil {
			return err
		}
	}

	return nil
}

func (l *CheckService) findServiceByName(ctx context.Context, service string) (name string) {
	output, _, _, err := l.snc.execCommand(ctx, systemctlNames, DefaultCmdTimeout)
	if err != nil {
		log.Tracef("systemctl failed: %s", err.Error())

		return ""
	}

	services := strings.Split(output, "\n")
	for _, svc := range services {
		match := reSvcNameLine.FindStringSubmatch(svc)
		if len(match) > 1 {
			realService := strings.TrimSpace(match[1])
			if strings.EqualFold(realService, service) {
				return realService
			}
		}
	}

	return ("")
}
