package snclient

import (
	"context"
	"fmt"

	"pkg/utils"
)

func init() {
	AvailableChecks["check_cpu"] = CheckEntry{"check_cpu", new(CheckCPU)}
}

type CheckCPU struct {
	times []string
}

func (l *CheckCPU) Build() *CheckData {
	l.times = []string{"5m", "1m", "5s"}

	return &CheckData{
		name:         "check_cpu",
		description:  "Checks the cpu usage metrics.",
		implemented:  ALL,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"time": {value: &l.times, description: "The times to check, default: 5m,1m,5s"},
		},
		defaultFilter:   "core = 'total'",
		defaultWarning:  "load > 80",
		defaultCritical: "load > 90",
		okSyntax:        "%(status): CPU load is ok.",
		detailSyntax:    "${time}: ${load}%",
		topSyntax:       "${status}: ${problem_list}",
		emptyState:      3,
		emptySyntax:     "check_cpu failed to find anything with this filter.",
		attributes: []CheckAttribute{
			{name: "core", description: "Core to check (total or core ##)"},
			{name: "core_id", description: "Core to check (total or core_##) "},
			{name: "idle", description: "Current idle load for a given core (currently not supported)"},
			{name: "kernel", description: "Current kernel load for a given core (currently not supported)"},
			{name: "load", description: "Current load for a given core"},
			{name: "time", description: "Time frame to check"},
		},
		exampleDefault: `
    check_cpu
    OK: CPU load is ok. |'total 5m'=13%;80;90 'total 1m'=13%;80;90 'total 5s'=13%;80;90

Checking **each core** by adding filter=none (disabling the filter):

    check_cpu filter=none
    OK: CPU load is ok. |'core4 5m'=13%;80;90 'core4 1m'=12%;80;90 'core4 5s'=9%;80;90 'core6 5m'=10%;80;90 'core6 1m'=10%;80;90 'core6 5s'=3%;80;90 'core5 5m'=10%;80;90 'core5 1m'=9%;80;90 'core5 5s'=6%;80;90 'core7 5m'=10%;80;90 'core7 1m'=10%;80;90 'core7 5s'=7%;80;90 'core1 5m'=13%;80;90 'core1 1m'=12%;80;90 'core1 5s'=10%;80;90 'core2 5m'=17%;80;90 'core2 1m'=17%;80;90 'core2 5s'=9%;80;90 'total 5m'=12%;80;90 'total 1m'=12%;80;90 'total 5s'=8%;80;90 'core3 5m'=12%;80;90 'core3 1m'=12%;80;90 'core3 5s'=11%;80;90 'core0 5m'=14%;80;90 'core0 1m'=14%;80;90 'core0 5s'=14%;80;90
	`,
		exampleArgs: `'warn=load > 80' 'crit=load > 95'`,
	}
}

func (l *CheckCPU) Check(_ context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	if len(snc.Counter.Keys("cpu")) == 0 {
		return nil, fmt.Errorf("no cpu counter available, make sure CheckSystem / CheckSystemUnix in /modules config is enabled")
	}

	for _, name := range snc.Counter.Keys("cpu") {
		if !check.MatchFilterMap(map[string]string{"core": name, "core_id": name}) {
			continue
		}

		counter := snc.Counter.Get("cpu", name)
		if counter == nil {
			continue
		}
		for _, time := range l.times {
			dur, _ := utils.ExpandDuration(time)
			avg := counter.AvgForDuration(dur)
			check.listData = append(check.listData, map[string]string{
				"time":    time,
				"core":    name,
				"core_id": name,
				"load":    fmt.Sprintf("%.0f", utils.ToPrecision(avg, 0)),
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
