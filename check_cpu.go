package snclient

import "fmt"

func init() {
	AvailableChecks["check_cpu"] = CheckEntry{"check_cpu", new(CheckCPU)}
}

type CheckCPU struct {
	noCopy noCopy
}

/* check_cpu */
func (l *CheckCPU) Check(_ []string) (*CheckResult, error) {
	state := CheckExitUnknown
	output := ""
	metrics := make([]*CheckMetric, 0)

	counter := agent.Counter.Get("cpu", "total")
	if counter != nil {
		dur, _ := ExpandDuration("15m")
		avg := counter.AvgForDuration(dur)
		output = fmt.Sprintf("OK - cpu total %1.f%%", avg)
	}

	return &CheckResult{
		State:   int64(state),
		Output:  output,
		Metrics: metrics,
	}, nil
}
