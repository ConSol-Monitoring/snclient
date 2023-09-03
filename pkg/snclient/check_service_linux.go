package snclient

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"pkg/humanize"

	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_service"] = CheckEntry{"check_service", new(CheckService)}
}

const systemctlTimeout = 30

var (
	reSvcDetails   = regexp.MustCompile(`(?s)^.*?\.service(?:\s\-\s(.*?)|)\n.*Active:\s*([A-Za-z() :-]+)(?:\ssince|\n)`)
	reSvcMainPid   = regexp.MustCompile(`Main\sPID:\s(\d+)`)
	reSvcPidMaster = regexp.MustCompile(`─(\d+).+\(master\)`)
	reSvcMemory    = regexp.MustCompile(`Memory:\s([\w.]+)`)
	reSvcPreset    = regexp.MustCompile(`\s+preset:\s+(\w+)\)`)
	reSvcSince     = regexp.MustCompile(`since\s+([^;]+);`)
	reSvcStatic    = regexp.MustCompile(`;\sstatic\)`)
)

type CheckService struct {
	snc      *Agent
	services []string
	excludes []string
}

func (l *CheckService) Build() *CheckData {
	l.services = []string{}
	l.excludes = []string{}

	return &CheckData{
		name:         "check_service",
		description:  "Checks the state of one or multiple linux (systemctl) services.",
		hasInventory: true,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]interface{}{
			"service": &l.services,
			"exclude": &l.excludes,
		},
		defaultFilter:   "none",
		defaultCritical: "state not in ('running', 'oneshot', 'static') && preset != 'disabled'",
		detailSyntax:    "${name}=${state}",
		topSyntax:       "%(status): %(crit_list)",
		okSyntax:        "%(status): All %(count) service(s) are ok.",
		emptySyntax:     "%(status): No services found",
		emptyState:      CheckExitUnknown,
	}
}

func (l *CheckService) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	l.snc = snc

	if len(l.services) == 0 || slices.Contains(l.services, "*") {
		output, stderr, _, _, err := snc.runExternalCommand(ctx, "systemctl --type=service --plain --no-pager --quiet", systemctlTimeout)
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

	return check.Finalize()
}

func (l *CheckService) addService(ctx context.Context, check *CheckData, service string, services, excludes []string) error {
	output, stderr, _, _, err := l.snc.runExternalCommand(ctx, fmt.Sprintf("systemctl status %s.service", service), systemctlTimeout)
	if err != nil {
		return fmt.Errorf("systemctl failed: %s\n%s", err.Error(), stderr)
	}

	if match, _ := regexp.MatchString(`Unit .* could not be found`, output); match || len(output) < 1 {
		return fmt.Errorf("could not find service: %s", service)
	}

	listEntry := l.parseSystemCtlStatus(service, output)

	if !l.isRequired(check, listEntry, services, excludes) {
		return nil
	}

	check.listData = append(check.listData, listEntry)

	if len(services) == 0 && !check.showAll {
		return nil
	}

	check.result.Metrics = append(check.result.Metrics, &CheckMetric{
		Name:  service,
		Value: l.svcStateFloat(listEntry["state"]),
	})
	memBytes, err := humanize.ParseBytes(listEntry["mem"])
	if err != nil {
		memBytes = 0
	}
	check.result.Metrics = append(
		check.result.Metrics,
		&CheckMetric{
			Name:  fmt.Sprintf("%s mem", service),
			Value: float64(memBytes),
			Unit:  "B",
		},
	)

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
		"preset":  "",
		"desc":    "",
		"pid":     "",
		"mem":     "",
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

	match = reSvcMemory.FindStringSubmatch(output)
	if len(match) > 1 {
		listEntry["mem"] = match[1]
	}

	match = reSvcPreset.FindStringSubmatch(output)
	if len(match) > 1 {
		listEntry["preset"] = match[1]
	}

	match = reSvcSince.FindStringSubmatch(output)
	if len(match) > 1 {
		listEntry["created"] = match[1]
	}

	match = reSvcStatic.FindStringSubmatch(output)
	if len(match) > 0 {
		listEntry["state"] = "static"
	}

	return listEntry
}
