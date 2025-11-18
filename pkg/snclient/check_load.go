package snclient

import (
	"cmp"
	"context"
	"fmt"
	"runtime"
	"slices"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/humanize"
	"github.com/consol-monitoring/snclient/pkg/utils"
	cpuinfo "github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/process"
)

func init() {
	AvailableChecks["check_load"] = CheckEntry{"check_load", NewCheckLoad}

	// starts a ticker (at least on windows to calculate averages)
	if runtime.GOOS == "windows" {
		go load.Avg() //nolint:errcheck // we do not want to log anything yet, if it continues to fail, it will be logged later
	}
}

type CheckLoad struct {
	// Warning threshold, triggered if the value goes above. Format: "WLOAD1,WLOAD5,WLOAD15"
	warning string
	// Critical threshold, triggered if the value goes above. Format: "CLOAD1,CLOAD5,CLOAD15"
	critical string
	// Divide load averages for the past 1,5,15 minutes by the number of cpus
	perCPU bool
	// List the top N cpu consuming processes
	numProcs int64
}

func NewCheckLoad() CheckHandler {
	return &CheckLoad{}
}

func (l *CheckLoad) Build() *CheckData {
	return &CheckData{
		name:         "check_load",
		description:  "Checks the cpu load metrics.",
		implemented:  ALL,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"-w|--warning":       {value: &l.warning, description: "Warning threshold: WLOAD1,WLOAD5,WLOAD15"},
			"-c|--critical":      {value: &l.critical, description: "Critical threshold: CLOAD1,CLOAD5,CLOAD15"},
			"-r|--percpu":        {value: &l.perCPU, description: "Divide the load averages by the number of CPUs"},
			"-n|--procs-to-show": {value: &l.numProcs, description: "Number of processes to show when printing the top consuming processes"},
		},
		defaultFilter: "none",
		detailSyntax:  "${type} load average: ${load1}, ${load5}, ${load15}",
		topSyntax:     "%(status) - ${list} on ${cores} cores",
		listCombine:   " - ",
		attributes: []CheckAttribute{
			{name: "type", description: "type will be either 'total' or 'scaled'"},
			{name: "load1", description: "average load value over 1 minute"},
			{name: "load5", description: "average load value over 5 minutes"},
			{name: "load15", description: "average load value over 15 minutes"},
			{name: "load", description: "maximum value of load1, load5 and load15"},
		},
		exampleDefault: `
    check_load
    OK - total load average: 2.36, 1.26, 1.01 |'load1'=2.36;;;0 'load5'=1.26;;;0 'load15'=1.01;;;0
	`,
		exampleArgs: `'warn=load > 20' 'crit=load > 30'`,
	}
}

func (l *CheckLoad) Check(ctx context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	loadStat, err := load.AvgWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("load.Avg(): %s", err.Error())
	}
	l.addLoadStats(check, "total", loadStat)

	//nolint:nestif // Anything less nested would be more complex to read?
	if !l.perCPU {
		// If the percpu mode is off, thresholds are only added for total load values
		warningThresholdTransformationError := l.transformPluginThresholds(l.warning, "W", "total", &check.warnThreshold)
		if warningThresholdTransformationError != nil {
			return nil, warningThresholdTransformationError
		}
		criticalThresholdTransformationError := l.transformPluginThresholds(l.critical, "C", "total", &check.critThreshold)
		if criticalThresholdTransformationError != nil {
			return nil, criticalThresholdTransformationError
		}

		// Add total load values as metrics
		l.addLoadMetrics(check, "", loadStat)

		// Scaled values are not added to the list data, no need to add them as metrics with null values
	} else {
		numCPU, err2 := cpuinfo.CountsWithContext(ctx, true)
		if err2 != nil {
			return nil, fmt.Errorf("cpuinfo: %s", err2.Error())
		}
		if numCPU == 0 {
			return nil, fmt.Errorf("cpu count is zero")
		}
		scaledLoadStat := &load.AvgStat{
			Load1:  loadStat.Load1 / float64(numCPU),
			Load5:  loadStat.Load5 / float64(numCPU),
			Load15: loadStat.Load15 / float64(numCPU),
		}
		// Add scaled load values to the list data
		l.addLoadStats(check, "scaled", scaledLoadStat)

		// In the percpu mode, thresholds are added only for the scaled load values
		warningThresholdTransformationError := l.transformPluginThresholds(l.warning, "W", "scaled", &check.warnThreshold)
		if warningThresholdTransformationError != nil {
			return nil, warningThresholdTransformationError
		}
		criticalThresholdTransformationError := l.transformPluginThresholds(l.critical, "C", "scaled", &check.critThreshold)
		if criticalThresholdTransformationError != nil {
			return nil, criticalThresholdTransformationError
		}

		// Add scaled load values as metrics
		l.addLoadMetrics(check, "scaled_", scaledLoadStat)

		// IMPORTANT: Add metrics for the total values with null values
		check.result.Metrics = append(check.result.Metrics,
			&CheckMetric{
				ThresholdName: "load1",
				Name:          "load1",
				Value:         loadStat.Load1,
				Warning:       nil,
				Critical:      nil,
				Min:           &Zero,
			},
			&CheckMetric{
				ThresholdName: "load5",
				Name:          "load5",
				Value:         loadStat.Load5,
				Warning:       nil,
				Critical:      nil,
				Min:           &Zero,
			},
			&CheckMetric{
				ThresholdName: "load15",
				Name:          "load15",
				Value:         loadStat.Load15,
				Warning:       nil,
				Critical:      nil,
				Min:           &Zero,
			},
		)
	}

	if l.numProcs > 0 {
		err = l.appendProcs(ctx, check)
		if err != nil {
			return nil, fmt.Errorf("procs: %s", err.Error())
		}
	}

	cores, err := cpuinfo.CountsWithContext(ctx, true)
	if err != nil {
		log.Warnf("cpuinfo.Counts: %s", err.Error())
	}
	check.details = map[string]string{
		"cores": fmt.Sprintf("%d", cores),
	}

	return check.Finalize()
}

func (l *CheckLoad) addLoadStats(check *CheckData, typename string, loadAvg *load.AvgStat) {
	check.listData = append(check.listData, map[string]string{
		"type":   typename,
		"load":   fmt.Sprintf("%.2f", utils.ToPrecision(max(loadAvg.Load1, loadAvg.Load5, loadAvg.Load15), 2)),
		"load1":  fmt.Sprintf("%.2f", utils.ToPrecision(loadAvg.Load1, 2)),
		"load5":  fmt.Sprintf("%.2f", utils.ToPrecision(loadAvg.Load5, 2)),
		"load15": fmt.Sprintf("%.2f", utils.ToPrecision(loadAvg.Load15, 2)),
	})
}

// typename is will be added as "type" attribute, is either "total" or "scaled"
// do not forget to put all conditions for the check before calling this function
func (l *CheckLoad) addLoadMetrics(check *CheckData, perfPrefix string, loadAvg *load.AvgStat) {
	// before adding, transform 'load' keyword into 'load1', 'load5' and 'load15'
	// this behavior is wanted, so that users can use 'load' keyword without caring which condition hits
	check.result.Metrics = append(check.result.Metrics,
		&CheckMetric{
			ThresholdName: "load1",
			Name:          perfPrefix + "load1",
			Value:         utils.ToPrecision(loadAvg.Load1, 2),
			Warning:       check.TransformMultipleKeywords([]string{"load"}, "load1", check.warnThreshold),
			Critical:      check.TransformMultipleKeywords([]string{"load"}, "load1", check.critThreshold),
			Min:           &Zero,
		},
		&CheckMetric{
			ThresholdName: "load5",
			Name:          perfPrefix + "load5",
			Value:         utils.ToPrecision(loadAvg.Load5, 2),
			Warning:       check.TransformMultipleKeywords([]string{"load"}, "load5", check.warnThreshold),
			Critical:      check.TransformMultipleKeywords([]string{"load"}, "load5", check.critThreshold),
			Min:           &Zero,
		},
		&CheckMetric{
			ThresholdName: "load15",
			Name:          perfPrefix + "load15",
			Value:         utils.ToPrecision(loadAvg.Load15, 2),
			Warning:       check.TransformMultipleKeywords([]string{"load"}, "load15", check.warnThreshold),
			Critical:      check.TransformMultipleKeywords([]string{"load"}, "load15", check.critThreshold),
			Min:           &Zero,
		})
}

func (l *CheckLoad) appendProcs(ctx context.Context, check *CheckData) error {
	format := "%-8s %-8s %-8s %-8s %-8s %-8s %-8s %s\n"
	check.result.Details = fmt.Sprintf(format,
		"USER", "PID", "%CPU", "%MEM", "VSC", "RSS", "TIME", "COMMAND")

	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return fmt.Errorf("fetching processes failed: %s", err.Error())
	}

	type sortableProc struct {
		cpuPercent float64
		proc       *process.Process
	}

	sortable := []sortableProc{}

	for _, proc := range procs {
		p, _ := proc.CPUPercentWithContext(ctx)
		sortable = append(sortable, sortableProc{
			cpuPercent: p,
			proc:       proc,
		})
	}

	slices.SortFunc(sortable, func(a, b sortableProc) int {
		return cmp.Compare(b.cpuPercent, a.cpuPercent)
	})

	for i, proc := range sortable {
		if i >= int(l.numProcs) {
			break
		}
		user, _ := proc.proc.Username()
		mem, _ := proc.proc.MemoryPercent()
		memInfo, _ := proc.proc.MemoryInfoWithContext(ctx)
		time, _ := proc.proc.TimesWithContext(ctx)
		cmdLine, _ := proc.proc.Cmdline()
		check.result.Details += fmt.Sprintf(format,
			user,
			fmt.Sprintf("%d", proc.proc.Pid),
			fmt.Sprintf("%.1f", proc.cpuPercent),
			fmt.Sprintf("%.1f", mem),
			humanize.Bytes(memInfo.VMS),
			humanize.Bytes(memInfo.RSS),
			fmt.Sprintf("%.1f", time.User+time.System),
			cmdLine,
		)
	}

	return nil
}

// transform "-w num,num,num" and "-c num,num,num" thresholds into ConditionLists
// Argument to the modifier threshold is given as a pointer, points either to warning or the critical
// typename is the 'type' of the attribute, either 'total' or 'scaled'
func (l *CheckLoad) transformPluginThresholds(thresholdString, prefix, typename string, threshold *ConditionList) error {
	if thresholdString == "" {
		return nil
	}
	splitted := strings.Split(thresholdString, ",")
	if len(splitted) == 1 {
		// use same threshold for 1m, 5m and 15m
		splitted = append(splitted, splitted[0], splitted[0])
	}
	if len(splitted) != 3 {
		return fmt.Errorf("warning threshold must be: %s1,%s5,%s15", prefix, prefix, prefix)
	}

	newThreshold := *threshold
	newThreshold = append(newThreshold,
		// The assumption is that these three conditions are used with a logical OR between them
		&Condition{
			group: []*Condition{
				{
					keyword:  "type",
					value:    typename,
					operator: Equal,
					unit:     "",
					original: "type == " + "'" + typename + "'",
				},
				{
					keyword:  "load1",
					value:    splitted[0],
					operator: Greater,
					unit:     "",
					original: "load1 > " + splitted[0],
				},
			},
			groupOperator: GroupAnd,
			original:      "type == " + "'" + typename + "' && load1 > " + splitted[0],
		},
		&Condition{
			group: []*Condition{
				{
					keyword:  "type",
					value:    typename,
					operator: Equal,
					unit:     "",
					original: "type == 'total'",
				},
				{
					keyword:  "load5",
					value:    splitted[1],
					operator: Greater,
					unit:     "",
					original: "load5 > " + splitted[1],
				},
			},
			groupOperator: GroupAnd,
			original:      "type == " + "'" + typename + "' && load5 > " + splitted[1],
		},
		&Condition{
			group: []*Condition{
				{
					keyword:  "type",
					value:    typename,
					operator: Equal,
					unit:     "",
					original: "type == 'total'",
				},
				{
					keyword:  "load15",
					value:    splitted[2],
					operator: Greater,
					unit:     "",
					original: "load15 > " + splitted[2],
				},
			},
			groupOperator: GroupAnd,
			original:      "type == " + "'" + typename + "' && load15 > " + splitted[2],
		},
	)
	*threshold = newThreshold

	return nil
}
