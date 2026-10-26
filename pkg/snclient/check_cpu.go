package snclient

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/consol-monitoring/snclient/pkg/utils"
	cpuinfo "github.com/shirou/gopsutil/v4/cpu"
)

var cpuKeyNum = regexp.MustCompile(`\d+$`)

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
		description:  "Checks the CPU usage metrics.",
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
		okSyntax:        "%(status) - CPU load is ok. %(total:fmt=%d)% on %(core_num) cores",
		detailSyntax:    "%(time): %(load)%",
		topSyntax:       "%(status) - %(problem_list) on %(core_num) cores",
		emptyState:      3,
		emptySyntax:     "check_cpu failed to find anything with this filter.",
		attributes: []CheckAttribute{
			{name: "core", description: "Core to check (total or core ##)"},
			{name: "core_id", description: "Core to check (total or core_##)"},
			{name: "core_nr", description: "Core number of the without prefix (e.g., core_1 -> 1)"},
			{name: "idle", description: "Current idle load for a given core (currently not supported)"},
			{name: "kernel", description: "Current kernel load for a given core (currently not supported)"},
			{name: "load", description: "Current load for a given core"},
			{name: "time", description: "Time frame checked"},
		},
		exampleDefault: `
    check_cpu
    OK - CPU load is ok. |'total 5s'=13.450%;80;90 'total 1m'=13.450%;80;90 'total 5m'=13.450%;80;90

Checking **each core** by adding filter=none (disabling the filter):

    check_cpu filter=none
    OK - CPU load is ok. |'core1 5s'=9.010%;80;90 'core1 1m'=12.340%;80;90 'core1 5m'=13.450%;80;90...
	`,
		exampleArgs: `'warn=load > 80' 'crit=load > 95'`,
	}
}

func (l *CheckCPU) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	numCPU, err := cpuinfo.CountsWithContext(ctx, true)
	if err != nil {
		log.Warnf("cpuinfo.Counts: %s", err.Error())
	}

	if len(snc.Counter.Keys("cpu")) == 0 {
		return nil, fmt.Errorf("no cpu counter available, make sure CheckSystem / CheckSystemUnix in /modules config is enabled")
	}

	var total float64
	times := sortedCPUTimes(l.times)
	// total: average load of the largest time range (last after ascending sort)
	maxIdx := len(times) - 1
	for _, name := range sortedCPUKeys(snc.Counter.Keys("cpu")) {
		coreNr := ""
		if matches := cpuKeyNum.FindStringSubmatch(name); len(matches) > 0 {
			coreNr = matches[0]
		}
		if res, ok := check.MatchFilterMap(map[string]string{"core": name, "core_id": name, "core_nr": coreNr}); !res && ok {
			continue
		}

		counter := snc.Counter.Get("cpu", name)
		if counter == nil {
			continue
		}
		for i, durStr := range times {
			dur, _ := utils.ExpandDuration(durStr)
			avg := counter.AvgForDuration(time.Duration(dur) * time.Second)
			if name == "total" && i == maxIdx {
				total = avg
			}
			check.listData = append(check.listData, map[string]string{
				"time":    durStr,
				"core":    name,
				"core_id": name,
				"core_nr": coreNr,
				"load":    fmt.Sprintf("%.3f", utils.ToPrecision(avg, 3)),
				"cores":   fmt.Sprintf("%d", numCPU),
			})
			check.result.Metrics = append(check.result.Metrics, &CheckMetric{
				ThresholdName: "load",
				Name:          fmt.Sprintf("%s %s", name, durStr),
				Value:         utils.ToPrecision(avg, 3),
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

	// Use the previously obtained numCPU instead of querying again
	check.details = map[string]string{
		"total":    fmt.Sprintf("%f", total),
		"core_num": fmt.Sprintf("%d", numCPU),
		"cores":    fmt.Sprintf("%d", numCPU),
	}

	if check.HasThreshold("cores") || check.HasFilter("cores") {
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			Name:          "cores",
			ThresholdName: "cores",
			Unit:          "",
			Value:         numCPU,
			Warning:       check.warnThreshold,
			Critical:      check.critThreshold,
			Min:           &Zero,
		})
	}

	return check.Finalize()
}

// sorts the time arguments by duration, ascending
func sortedCPUTimes(times []string) []string {
	slices.SortStableFunc(times, func(a, b string) int {
		da, _ := utils.ExpandDuration(a)
		db, _ := utils.ExpandDuration(b)

		return int(da - db)
	})

	return times
}

// sorts counter keys with "total" first, then core keys numerically (core10 after core9)
func sortedCPUKeys(keys []string) []string {
	slices.SortStableFunc(keys, func(a, b string) int {
		switch {
		case a == "total":
			return -1
		case b == "total":
			return 1
		}
		numA, _ := strconv.Atoi(cpuKeyNum.FindString(a))
		numB, _ := strconv.Atoi(cpuKeyNum.FindString(b))

		return numA - numB
	})

	return keys
}
