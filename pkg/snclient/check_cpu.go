package snclient

import (
	"fmt"
	"strings"

	"pkg/utils"
)

func init() {
	AvailableChecks["check_cpu"] = CheckEntry{"check_cpu", new(CheckCPU)}
}

type CheckCPU struct{}

/* check_cpu */
func (l *CheckCPU) Check(snc *Agent, args []string) (*CheckResult, error) {
	check := &CheckData{
		result: &CheckResult{
			State: CheckExitOK,
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
	argList, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
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

	for _, name := range snc.Counter.Keys("cpu") {
		if !check.MatchFilter("core", name) {
			continue
		}

		counter := snc.Counter.Get("cpu", name)
		if counter == nil {
			continue
		}
		for _, time := range times {
			time = strings.TrimSpace(time)
			dur, _ := utils.ExpandDuration(time)
			avg := counter.AvgForDuration(dur)
			check.listData = append(check.listData, map[string]string{
				"load": fmt.Sprintf("%f", avg),
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
