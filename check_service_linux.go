package snclient

import (
	"fmt"
	"strconv"
	"regexp"

	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_service"] = CheckEntry{"check_service", new(CheckService)}
}

type CheckService struct {
	noCopy noCopy
}

var SERVICE_STATE = map[string]string{
	"stopped":      "1",
	"dead":         "1",
	"startpending": "2",
	"stoppending":  "3",
	"running":      "4",
	"started":      "4",
}

/* check_service todo
 * todo
 */
func (l *CheckService) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(0)
	argList := ParseArgs(args)
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
	out, err := exec.Command("systemctl", "status", fmt.Sprintf("%s.service", service).Output()
	if err != nil {
		return &CheckResult{
			State:  int64(3),
			Output: fmt.Sprintf("Service %s not found: %s", service, err),
		}, nil
	}

	re := regexp.MustCompile(`Active:\s*[A-Za-z]+\s*\(([A-Za-z]+)\)`)
	match := re.FindStringSubmatch(string(out))
	
	warnTreshold.value = SERVICE_STATE[warnTreshold.value]
	critTreshold.value = SERVICE_STATE[critTreshold.value]

	mdata := []MetricData{{name: "state", value: SERVICE_STATE[match[1]], 10)}}

	// compare ram metrics to tresholds
	if CompareMetrics(mdata, warnTreshold) {
		state = CheckExitWarning
	}

	if CompareMetrics(mdata, critTreshold) {
		state = CheckExitCritical
	}

	output := "Service is ok."

	return &CheckResult{
		State:  state,
		Output: output,
		Metrics: []*CheckMetric{
			{
				Name:  service,
				Value: float64(statusCode.State),
			},
		},
	}, nil
}
