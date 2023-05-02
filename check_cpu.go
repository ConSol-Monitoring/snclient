package snclient

import (
	"fmt"
	"strings"

	"pkg/utils"
)

func init() {
	AvailableChecks["check_cpu"] = CheckEntry{"check_cpu", new(CheckCPU)}
}

type CheckCPU struct {
	noCopy noCopy
	data   CheckData
}

/* check_cpu */
func (l *CheckCPU) Check(args []string) (*CheckResult, error) {
	state := CheckExitOK
	output := "CPU load is ${status_lc}. "
	metrics := make([]*CheckMetric, 0)
	argList, err := ParseArgs(args, &l.data)
	if err != nil {
		return nil, fmt.Errorf("args error: %s", err.Error())
	}

	times := []string{}
	for _, a := range argList {
		if a.key == "time" {
			times = append(times, strings.Split(a.value, ",")...)
		}
	}

	if len(times) == 0 {
		times = []string{"5m", "1m", "5s"}
	}
	names := []string{"total"}

	for _, name := range names {
		counter := agent.Counter.Get("cpu", name)
		if counter != nil {
			for _, time := range times {
				time = strings.TrimSpace(time)
				dur, _ := utils.ExpandDuration(time)
				avg := counter.AvgForDuration(dur)
				metrics = append(metrics, &CheckMetric{
					Name:  fmt.Sprintf("%s %s", name, time),
					Value: avg,
					Unit:  "%",
				})
				compare := map[string]string{"load": fmt.Sprintf("%f", avg)}
				switch {
				case CompareMetrics(compare, l.data.critThreshold):
					state = CheckExitCritical
				case CompareMetrics(compare, l.data.warnThreshold) && state < CheckExitWarning:
					state = CheckExitWarning
				}
			}
		}
	}

	return &CheckResult{
		State:   int64(state),
		Output:  output,
		Metrics: metrics,
	}, nil
}
