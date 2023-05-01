package snclient

import (
	"fmt"

	"pkg/utils"
)

func init() {
	AvailableChecks["check_cpu"] = CheckEntry{"check_cpu", new(CheckCPU)}
}

type CheckCPU struct {
	noCopy noCopy
}

/* check_cpu */
func (l *CheckCPU) Check(_ []string) (*CheckResult, error) {
	state := CheckExitUnknown
	output := "OK: CPU load is ok. "
	metrics := make([]*CheckMetric, 0)

	times := []string{"5m", "1m", "5s"}
	names := []string{"total"}

	for _, name := range names {
		counter := agent.Counter.Get("cpu", name)
		if counter != nil {
			for _, time := range times {
				dur, _ := utils.ExpandDuration(time)
				avg := counter.AvgForDuration(dur)
				metrics = append(metrics, &CheckMetric{
					Name:  fmt.Sprintf("%s %s", name, time),
					Value: avg,
					Unit:  "%",
				})
			}
		}
	}

	return &CheckResult{
		State:   int64(state),
		Output:  output,
		Metrics: metrics,
	}, nil
}
