package snclient

import (
	"fmt"

	"pkg/utils"
)

func init() {
	AvailableChecks["check_cpu"] = CheckEntry{"check_cpu", new(CheckCPU)}
}

type CheckCPU struct{}

func (l *CheckCPU) Check(snc *Agent, args []string) (*CheckResult, error) {
	times := []string{"5m", "1m", "5s"}
	check := &CheckData{
		name:        "check_cpu",
		description: "Checks the cpu usage metrics.",
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]interface{}{
			"time": &times,
		},
		defaultFilter:   "core = 'total'",
		defaultWarning:  "load > 80",
		defaultCritical: "load > 90",
		okSyntax:        "%(status): CPU load is ok.",
		detailSyntax:    "${time}: ${load}%",
		topSyntax:       "${status}: ${problem_list}",
		emptyState:      3,
		emptySyntax:     "check_cpu failed to find anything with this filter.",
	}
	_, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	if len(snc.Counter.Keys("cpu")) == 0 {
		return nil, fmt.Errorf("no cpu counter available, make sure CheckSystem / CheckSystemUnix in /modules config is enabled")
	}

	for _, name := range snc.Counter.Keys("cpu") {
		if !check.MatchFilter("core", name) {
			continue
		}

		counter := snc.Counter.Get("cpu", name)
		if counter == nil {
			continue
		}
		for _, time := range times {
			dur, _ := utils.ExpandDuration(time)
			avg := counter.AvgForDuration(dur)
			check.listData = append(check.listData, map[string]string{
				"time": time,
				"core": name,
				"load": fmt.Sprintf("%.0f", utils.ToPrecision(avg, 0)),
			})
			check.result.Metrics = append(check.result.Metrics, &CheckMetric{
				ThresholdName: "load",
				Name:          fmt.Sprintf("%s %s", name, time),
				Value:         utils.ToPrecision(avg, 0),
				Unit:          "%",
				Warning:       check.warnThreshold,
				Critical:      check.critThreshold,
			})
		}
	}

	return check.Finalize()
}
