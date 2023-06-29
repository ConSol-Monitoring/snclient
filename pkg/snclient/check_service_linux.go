package snclient

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"pkg/humanize"

	"golang.org/x/exp/slices"
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
		detailSyntax:    "${name}=${state}",
		topSyntax:       "%(status): %(crit_list), delayed (%(warn_list))",
		okSyntax:        "%(status): All %(count) service(s) are ok.",
		emptySyntax:     "%(status): No services found",
		emptyState:      CheckExitUnknown,
	}
	_, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	out, err := exec.Command("systemctl", "--type=service").Output()
	if err != nil {
		return &CheckResult{
			State:  int64(3),
			Output: fmt.Sprintf("Failed to fetch service list: %s", err),
		}, nil
	}

	re := regexp.MustCompile(`([\w\-@.]+)\.service`)
	matches := re.FindAllStringSubmatch(string(out), -1)

	serviceList := []string{}

	for _, match := range matches {
		serviceList = append(serviceList, match[1])
	}

	for _, service := range serviceList {
		if slices.Contains(excludes, service) {
			log.Tracef("service %s excluded by 'exclude' argument", service)

			continue
		}

		err = l.addService(check, service, services, excludes)
		if err != nil {
			return nil, err
		}
	}

	for _, service := range services {
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

		err = l.addService(check, service, services, excludes)
		if err != nil {
			return nil, err
		}
	}

	return check.Finalize()
}

func (l *CheckService) addService(check *CheckData, service string, services, excludes []string) error {
	out, err := exec.Command("systemctl", "status", fmt.Sprintf("%s.service", service)).Output()

	if match, _ := regexp.MatchString(`Unit .* could not be found`, string(out)); match || len(out) < 1 {
		return fmt.Errorf("Couldnt find service: %s", service)
	}

	listEntry := map[string]string{
		"name":    service,
		"service": service,
	}

	re := regexp.MustCompile(`\.service - (.*)(?s).*Active:\s*([A-Za-z() :-]+)\ssince\s\w+\s(.*)\s\w+;`)
	match := re.FindStringSubmatch(string(out))
	if len(match) > 3 {
		listEntry["desc"] = match[1]
		listEntry["state"] = l.svcState(match[2])
		listEntry["created"] = match[3]
	} else {
		return fmt.Errorf("Couldnt retrieve metrics for service: %s", service)
	}

	re = regexp.MustCompile(`Main\sPID:\s(\d+)`)
	match = re.FindStringSubmatch(string(out))
	if len(match) < 1 {
		re = regexp.MustCompile(`─(\d+).+\(master\)`)
		match = re.FindStringSubmatch(string(out))
		if len(match) < 1 {
			listEntry["pid"] = "-1"
		} else {
			listEntry["pid"] = match[1]
		}
	} else {
		listEntry["pid"] = match[1]
	}

	re = regexp.MustCompile(`Memory:\s([\w.]+)`)
	match = re.FindStringSubmatch(string(out))
	if len(match) > 1 {
		listEntry["mem"] = match[1]
	} else {
		listEntry["mem"] = "-1"
	}

	if !l.isRequired(check, listEntry, services, excludes) {
		return nil
	}

	check.listData = append(check.listData, listEntry)

	if len(services) == 0 {
		return nil
	}

	check.result.Metrics = append(check.result.Metrics, &CheckMetric{
		Name:  service,
		Value: l.svcStateFloat(listEntry["state"]),
	})
	mem_bytes, err := humanize.ParseBytes(listEntry["mem"])
	if err != nil {
		mem_bytes = 0
	}
	check.result.Metrics = append(
		check.result.Metrics,
		&CheckMetric{
			Name:  fmt.Sprintf("%s mem", service),
			Value: float64(mem_bytes),
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
		return "exited"
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
	case "exited":
		return float64(3)
	case "running":
		return float64(4)
	}

	return float64(0)
}
