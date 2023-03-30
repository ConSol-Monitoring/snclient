package snclient

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
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

/* check_service todo
 * todo .
 */
func (l *CheckService) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(0)
	argList := ParseArgs(args)
	output := "Service is ok."
	var warnTreshold Treshold
	var critTreshold Treshold
	var service string

	// parse treshold args
	for _, arg := range argList {
		switch arg.key {
		case "warn", "warning":
			warnTreshold = ParseTreshold(arg.value)
		case "crit", "critical":
			critTreshold = ParseTreshold(arg.value)
		case "service":
			service = arg.value
		}
	}

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

	warnTreshold.value = ServiceStates[warnTreshold.value]
	critTreshold.value = ServiceStates[critTreshold.value]

	mdata := []MetricData{{name: "state", value: stateStr}}

	// compare ram metrics to tresholds
	if CompareMetrics(mdata, warnTreshold) {
		state = CheckExitWarning
		output = "Service is warning!"
	}

	if CompareMetrics(mdata, critTreshold) {
		state = CheckExitCritical
		output = "Service is critical!"
	}

	return &CheckResult{
		State:  state,
		Output: output,
		Metrics: []*CheckMetric{
			{
				Name:  service,
				Value: stateFloat,
			},
		},
	}, nil
}
