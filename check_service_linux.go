package snclient

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func init() {
	AvailableChecks["check_service"] = CheckEntry{"check_service", new(CheckService)}
}

type CheckService struct {
	noCopy noCopy
}

var ServiceStates = map[string]string{
	"stopped":      "1",
	"dead":         "1",
	"startpending": "2",
	"stoppending":  "3",
	"running":      "4",
	"started":      "4",
}

/* check_service_linux
 * Description: Checks the state of a service on the host.
 * Tresholds: status
 * Units: stopped, dead, startpending, stoppedpending, running, started
 */
func (l *CheckService) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(0)
	argList := ParseArgs(args)
	output := "Service is ok."
	var warnTreshold Treshold
	var critTreshold Treshold
	var services []string
	detailSyntax := "%(service)=%(state)"
	topSyntax := "%(crit_list), delayed (%(warn_list))"
	okSyntax := "All %(count) service(s) are ok."
	var okList []string
	var warnList []string
	var critList []string
	var checkData map[string]string
	var metrics []*CheckMetric

	// parse treshold args
	for _, arg := range argList {
		switch arg.key {
		case "warn", "warning":
			warnTreshold = ParseTreshold(arg.value)
		case "crit", "critical":
			critTreshold = ParseTreshold(arg.value)
		case "service":
			services = append(services, arg.value)
		case "detail-syntax":
			detailSyntax = arg.value
		case "top-syntax":
			topSyntax = arg.value
		case "ok-syntax":
			okSyntax = arg.value
		}
	}

	warnTreshold.value = ServiceStates[warnTreshold.value]
	critTreshold.value = ServiceStates[critTreshold.value]

	for _, service := range services {
		// collect service state
		out, err := exec.Command("systemctl", "status", fmt.Sprintf("%s.service", service)).Output()
		if err != nil {
			return &CheckResult{
				State:  int64(3),
				Output: fmt.Sprintf("Service %s not found: %s", service, err),
			}, nil
		}

		re := regexp.MustCompile(`Active:\s*[A-Za-z]+\s*\(([A-Za-z]+)\)`)
		match := re.FindStringSubmatch(string(out))

		stateStr := ServiceStates[match[1]]
		stateFloat, _ := strconv.ParseFloat(stateStr, 64)

		metrics = append(metrics, &CheckMetric{Name: service, Value: stateFloat})

		mdata := []MetricData{{name: "state", value: stateStr}}
		sdata := map[string]string{
			"service": service,
			"state":   stateStr,
		}

		// compare ram metrics to tresholds
		if CompareMetrics(mdata, critTreshold) {
			critList = append(critList, ParseSyntax(detailSyntax, sdata))

			continue
		}

		if CompareMetrics(mdata, warnTreshold) {
			warnList = append(warnList, ParseSyntax(detailSyntax, sdata))

			continue
		}

		okList = append(okList, ParseSyntax(detailSyntax, sdata))
	}

	if len(critList) > 0 {
		state = CheckExitCritical
	} else if len(warnList) > 0 {
		state = CheckExitWarning
	} else {
		state = CheckExitOK
	}

	checkData = map[string]string{
		"status":    strconv.FormatInt(state, 10),
		"count":     strconv.FormatInt(int64(len(services)), 10),
		"ok_list":   strings.Join(okList, ", "),
		"warn_list": strings.Join(warnList, ", "),
		"crit_list": strings.Join(critList, ", "),
	}

	if state == CheckExitOK {
		output = ParseSyntax(okSyntax, checkData)
	} else {
		output = ParseSyntax(topSyntax, checkData)
	}

	return &CheckResult{
		State:   state,
		Output:  output,
		Metrics: metrics,
	}, nil
}
