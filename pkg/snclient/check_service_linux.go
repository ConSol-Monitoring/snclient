package snclient

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_service"] = CheckEntry{"check_service", NewCheckService}
}

var (
	reSvcDetails   = regexp.MustCompile(`(?s)^.*?\.service(?:\s\-\s(.*?)|)\n.*Active:\s*([A-Za-z() :-]+)(?:\ssince|\n)`)
	reSvcMainPid   = regexp.MustCompile(`Main\sPID:\s(\d+)`)
	reSvcPidMaster = regexp.MustCompile(`─(\d+).+\(master\)`)
	reSvcPreset    = regexp.MustCompile(`\s+preset:\s+(\w+)\)`)
	reSvcTasks     = regexp.MustCompile(`Tasks:\s*(\d+)`)
	reSvcStatic    = regexp.MustCompile(`;\sstatic\)`)
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
	return &CheckData{
		name: "check_service",
		description: `Checks the state of one or multiple linux (systemctl) services.

There is a specific [check_service for windows](check_service_windows) as well.`,
		implemented:  Linux,
		docTitle:     "service (linux)",
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"service": {value: &l.services, description: "Name of the service to check (set to * to check all services). Default: *"},
			"exclude": {value: &l.excludes, description: "List of services to exclude from the check (mainly used when service is set to *)"},
		},
		defaultFilter:   "none",
		defaultCritical: "state not in ('running', 'oneshot', 'static') && preset != 'disabled'",
		detailSyntax:    "${name}=${state}",
		topSyntax:       "%(status): %(crit_list)",
		okSyntax:        "%(status): All %(count) service(s) are ok.",
		emptySyntax:     "%(status): No services found",
		emptyState:      CheckExitUnknown,
		attributes: []CheckAttribute{
			{name: "name", description: "The name of the service"},
			{name: "service", description: "Alias for name"},
			{name: "desc", description: "Description of the service"},
			{name: "state", description: "The state of the service, one of: stopped, starting, oneshot, running or unknown"},
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
    check_service
    OK: All 74 service(s) are ok.

Or check a specific service and get some metrics:

    check_service service=docker ok-syntax='${top-syntax}' top-syntax='%(status): %(list)' detail-syntax='%(name) %(state) - memory: %(rss:h)B - age: %(age:duration)'
    OK: docker running - memory: 805.2MB - created: Fri 2023-11-17 20:34:01 CET |'docker'=4 'docker rss'=805200000B

Check memory usage of specific service:

    check_service service=docker warn='rss > 1GB' warn='rss > 2GB'
    OK: All 1 service(s) are ok. |'docker'=4 'docker rss'=59691008B;;;0 'docker vms'=3166244864B;;;0 'docker cpu'=0.7%;;;0 'docker tasks'=20;;;0
	`,
		exampleArgs: "service=docker",
	}
}

func (l *CheckService) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	l.snc = snc

	if len(l.services) == 0 || slices.Contains(l.services, "*") {
		output, stderr, _, _, err := snc.runExternalCommandString(ctx, "systemctl --type=service --plain --no-pager --quiet", DefaultCmdTimeout)
		if err != nil {
			return &CheckResult{
				State:  CheckExitUnknown,
				Output: fmt.Sprintf("Failed to fetch service list: %s%s", err.Error(), stderr),
			}, nil
		}

		re := regexp.MustCompile(`(?m)^(\S+)\.service\s+`)
		matches := re.FindAllStringSubmatch(output, -1)

		serviceList := []string{}
		for _, match := range matches {
			serviceList = append(serviceList, match[1])
		}
		for _, service := range serviceList {
			if slices.Contains(l.excludes, service) {
				log.Tracef("service %s excluded by 'exclude' argument", service)

				continue
			}

			err = l.addService(ctx, check, service, l.services, l.excludes)
			if err != nil {
				return nil, err
			}
		}
	}

	// add user supplied services not yet added
	for _, service := range l.services {
		if service == "*" {
			continue
		}
		found := false
		for _, e := range check.listData {
			if e["name"] == service {
				found = true

				break
			}
		}
		if found {
			continue
		}

		err := l.addService(ctx, check, service, l.services, l.excludes)
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

func (l *CheckService) addService(ctx context.Context, check *CheckData, service string, services, excludes []string) error {
	output, stderr, _, _, err := l.snc.runExternalCommandString(ctx, fmt.Sprintf("systemctl status %s.service", service), DefaultCmdTimeout)
	if err != nil {
		return fmt.Errorf("systemctl failed: %s\n%s", err.Error(), stderr)
	}

	if match, _ := regexp.MatchString(`Unit .* could not be found`, output); match || len(output) < 1 {
		return fmt.Errorf("could not find service: %s", service)
	}

	listEntry := l.parseSystemCtlStatus(service, output)

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
	if len(match) < 1 {
		match = reSvcPidMaster.FindStringSubmatch(output)
		if len(match) < 1 {
			listEntry["pid"] = ""
		} else {
			listEntry["pid"] = match[1]
		}
	} else {
		listEntry["pid"] = match[1]
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

	return listEntry
}
