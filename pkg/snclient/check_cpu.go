package snclient

import (
	"context"
	"fmt"
	"time"

	"github.com/consol-monitoring/snclient/pkg/utils"
	cpuinfo "github.com/shirou/gopsutil/v4/cpu"
)

func init() {
	AvailableChecks["check_cpu"] = CheckEntry{"check_cpu", NewCheckCPU}
}

type CheckCPU struct {
	times []string
	// List the top N cpu consuming processes
	numProcs int64
	// Show arguments when listing the top N processes
	showArgs bool
}

func NewCheckCPU() CheckHandler {
	return &CheckCPU{
		times: []string{"5m", "1m", "5s"},
	}
}

func (l *CheckCPU) Build() *CheckData {
	return &CheckData{
		name:         "check_cpu",
		description:  "Checks the cpu usage metrics.",
		implemented:  ALL,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"time":            {value: &l.times, description: "The times to check, default: 5m,1m,5s"},
			"n|procs-to-show": {value: &l.numProcs, description: "Number of processes to show when printing the top consuming processes"},
			"show-args":       {value: &l.showArgs, description: "Show arguments when listing the top N processes"},
		},
		defaultFilter:   "core = 'total'",
		defaultWarning:  "load > 80",
		defaultCritical: "load > 90",
		okSyntax:        "%(status) - CPU load is ok. %{total:fmt=%d}% on %{core_num} cores",
		detailSyntax:    "${time}: ${load}%",
		topSyntax:       "%(status) - ${problem_list} on %{core_num} cores",
		emptyState:      3,
		emptySyntax:     "check_cpu failed to find anything with this filter.",
		attributes: []CheckAttribute{
			{name: "core", description: "Core to check (total or core ##)"},
			{name: "core_id", description: "Core to check (total or core_##) "},
			{name: "idle", description: "Current idle load for a given core (currently not supported)"},
			{name: "kernel", description: "Current kernel load for a given core (currently not supported)"},
			{name: "load", description: "Current load for a given core"},
			{name: "time", description: "Time frame checked"},
		},
		exampleDefault: `
    check_cpu
    OK - CPU load is ok. |'total 5m'=13%;80;90 'total 1m'=13%;80;90 'total 5s'=13%;80;90

Checking **each core** by adding filter=none (disabling the filter):

    check_cpu filter=none
    OK - CPU load is ok. |'core1 5m'=13%;80;90 'core1 1m'=12%;80;90 'core1 5s'=9%;80;90...
	`,
		exampleArgs: `'warn=load > 80' 'crit=load > 95'`,
	}
}

func (l *CheckCPU) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	if len(snc.Counter.Keys("cpu")) == 0 {
		return nil, fmt.Errorf("no cpu counter available, make sure CheckSystem / CheckSystemUnix in /modules config is enabled")
	}

	var total float64
	for _, name := range snc.Counter.Keys("cpu") {
		if res, ok := check.MatchFilterMap(map[string]string{"core": name, "core_id": name}); !res && ok {
			continue
		}

		counter := snc.Counter.Get("cpu", name)
		if counter == nil {
			continue
		}
		for i, durStr := range l.times {
			dur, _ := utils.ExpandDuration(durStr)
			avg := counter.AvgForDuration(time.Duration(dur) * time.Second)
			if i == 0 {
				total = avg
			}
			check.listData = append(check.listData, map[string]string{
				"time":    durStr,
				"core":    name,
				"core_id": name,
				"load":    fmt.Sprintf("%.0f", utils.ToPrecision(avg, 0)),
			})
			check.result.Metrics = append(check.result.Metrics, &CheckMetric{
				ThresholdName: "load",
				Name:          fmt.Sprintf("%s %s", name, durStr),
				Value:         utils.ToPrecision(avg, 0),
				Unit:          "%",
				Warning:       check.warnThreshold,
				Critical:      check.critThreshold,
			})
		}
	}

	if l.numProcs > 0 {
		err := appendProcs(ctx, check, l.numProcs, l.showArgs, "cpu")
		if err != nil {
			return nil, fmt.Errorf("procs: %s", err.Error())
		}
	}

	cores, err := cpuinfo.CountsWithContext(ctx, true)
	if err != nil {
		log.Warnf("cpuinfo.Counts: %s", err.Error())
	}
	check.details = map[string]string{
		"total":    fmt.Sprintf("%f", total),
		"core_num": fmt.Sprintf("%d", cores),
	}

	return check.Finalize()
}
