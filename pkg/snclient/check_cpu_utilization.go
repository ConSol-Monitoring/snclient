package snclient

import (
	"context"
	"fmt"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/utils"
	cpuinfo "github.com/shirou/gopsutil/v4/cpu"
)

func init() {
	AvailableChecks["check_cpu_utilization"] = CheckEntry{"check_cpu_utilization", NewCheckCPUUtilization}
}

type CPUUtilizationResult struct {
	total  float64
	user   float64
	system float64
	iowait float64
	steal  float64
	guest  float64
	idle   float64
}

type CheckCPUUtilization struct {
	snc      *Agent
	avgRange string
}

func NewCheckCPUUtilization() CheckHandler {
	return &CheckCPUUtilization{
		avgRange: "1m",
	}
}

func (l *CheckCPUUtilization) Build() *CheckData {
	return &CheckData{
		name:         "check_cpu_utilization",
		description:  "Checks the cpu utilization metrics.",
		implemented:  ALL,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"range": {value: &l.avgRange, description: "Sets time range to calculate average (default is 1m)"},
		},
		defaultWarning:  "total > 90",
		defaultCritical: "total > 95",
		topSyntax:       "${status} - ${list}",
		detailSyntax:    "user: ${user}% - system: ${system}% - iowait: ${iowait}% - steal: ${steal}% - guest: ${guest} - idle: %{idle}%",
		attributes: []CheckAttribute{
			{name: "total", description: "Sum of user,system,iowait,steal and guest in percent", unit: UPercent},
			{name: "user", description: "User cpu utilization in percent", unit: UPercent},
			{name: "system", description: "System cpu utilization in percent", unit: UPercent},
			{name: "iowait", description: "IOWait cpu utilization in percent", unit: UPercent},
			{name: "steal", description: "Steal cpu utilization in percent", unit: UPercent},
			{name: "guest", description: "Guest cpu utilization in percent", unit: UPercent},
			{name: "idle", description: "Idle cpu utilization in percent", unit: UPercent},
		},
		exampleDefault: `
	check_cpu_utilization
OK - user: 2% - system: 1% - iowait: 0% - steal: 0% - guest: 0 - idle: 96% |'total'=3.4%;90;95;0; 'user'=2.11%;;;0;...
	`,
		exampleArgs: `'warn=total > 90%' 'crit=total > 95%'`,
	}
}

func (l *CheckCPUUtilization) Check(_ context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	l.snc = snc
	if len(snc.Counter.Keys("cpu")) == 0 {
		return nil, fmt.Errorf("no cpu counter available, make sure CheckSystem / CheckSystemUnix in /modules config is enabled")
	}

	lookBack, err := utils.ExpandDuration(l.avgRange)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse range: %s", err.Error())
	}
	if lookBack < 0 {
		lookBack *= -1
	}
	scanLookBack := uint64(lookBack)

	l.addCPUUtilizationMetrics(check, scanLookBack)

	return check.Finalize()
}

//nolint:funlen // The function is simple enough, the length comes from many fields to add.
func (l *CheckCPUUtilization) addCPUUtilizationMetrics(check *CheckData, scanLookBack uint64) {
	entry := map[string]string{
		"total":  "0",
		"user":   "0",
		"system": "0",
		"iowait": "0",
		"steal":  "0",
		"guest":  "0",
		"idle":   "0",
	}
	check.listData = append(check.listData, entry)

	cpuMetrics, ok := l.getMetrics(scanLookBack)
	if !ok {
		return
	}

	entry["total"] = fmt.Sprintf("%.f", cpuMetrics.total)
	entry["user"] = fmt.Sprintf("%.f", cpuMetrics.user)
	entry["system"] = fmt.Sprintf("%.f", cpuMetrics.system)
	entry["iowait"] = fmt.Sprintf("%.f", cpuMetrics.iowait)
	entry["steal"] = fmt.Sprintf("%.f", cpuMetrics.steal)
	entry["guest"] = fmt.Sprintf("%.f", cpuMetrics.guest)
	entry["idle"] = fmt.Sprintf("%.f", cpuMetrics.idle)

	check.result.Metrics = append(check.result.Metrics,
		&CheckMetric{
			Name:     "total",
			Value:    utils.ToPrecision(cpuMetrics.total, 2),
			Unit:     "%",
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
			Min:      &Zero,
		},
		&CheckMetric{
			Name:     "user",
			Value:    utils.ToPrecision(cpuMetrics.user, 2),
			Unit:     "%",
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
			Min:      &Zero,
		},
		&CheckMetric{
			Name:     "system",
			Value:    utils.ToPrecision(cpuMetrics.system, 2),
			Unit:     "%",
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
			Min:      &Zero,
		},
		&CheckMetric{
			Name:     "iowait",
			Value:    utils.ToPrecision(cpuMetrics.iowait, 2),
			Unit:     "%",
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
			Min:      &Zero,
		},
		&CheckMetric{
			Name:     "steal",
			Value:    utils.ToPrecision(cpuMetrics.steal, 2),
			Unit:     "%",
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
			Min:      &Zero,
		},
		&CheckMetric{
			Name:     "guest",
			Value:    utils.ToPrecision(cpuMetrics.guest, 2),
			Unit:     "%",
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
			Min:      &Zero,
		},
		&CheckMetric{
			Name:     "idle",
			Value:    utils.ToPrecision(cpuMetrics.idle, 2),
			Unit:     "%",
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
			Min:      &Zero,
		},
	)
}

func (l *CheckCPUUtilization) getMetrics(scanLookBack uint64) (res *CPUUtilizationResult, ok bool) {
	res = &CPUUtilizationResult{}

	counter1 := l.snc.Counter.Get("cpuinfo", "info")
	counter2 := l.snc.Counter.Get("cpuinfo", "info")
	if counter1 == nil || counter2 == nil {
		return nil, false
	}

	scanLookBack64, err := convert.Int64E(scanLookBack)
	if err != nil {
		log.Warnf("failed to convert scan look back: %s", err.Error())

		return nil, false
	}

	cpuinfo1 := counter1.GetLast()
	cpuinfo2 := counter2.GetAt(time.Now().Add(-time.Duration(scanLookBack64) * time.Second))
	if cpuinfo1 == nil || cpuinfo2 == nil {
		log.Errorf("Either the latest cpuinfo counter, or the cpuinfo counter from %d seconds ago seem to be null", scanLookBack)

		return nil, false
	}

	if cpuinfo1.UnixMilli < cpuinfo2.UnixMilli {
		log.Errorf("The last cpuinfo counters have a smaller timestamp: %d than the one that was found near %d seconds ago: %d", cpuinfo1.UnixMilli, scanLookBack, cpuinfo2.UnixMilli)

		return nil, false
	}
	duration := float64(cpuinfo1.UnixMilli - cpuinfo2.UnixMilli)

	if duration <= 0 {
		// This case might happen if there is not enough recorded counters to make up the look back time
		// We need to wait until that duration difference can be achieved
		secondsToSleep := min(scanLookBack, 5)

		log.Tracef("Waiting %d seconds and returning that value, as cpu utilization metrics for the last %d seconds is not available yet.", secondsToSleep, scanLookBack)
		time.Sleep(time.Second * time.Duration(convert.Int32(secondsToSleep)))

		return l.getMetrics(secondsToSleep - 1)
	}
	duration /= 1e3 // cpu times are measured in seconds

	info1, ok := cpuinfo1.Value.(*cpuinfo.TimesStat)
	if !ok {
		return nil, false
	}
	info2, ok := cpuinfo2.Value.(*cpuinfo.TimesStat)
	if !ok {
		return nil, false
	}

	numCPU, err := cpuinfo.Counts(true)
	if err != nil {
		log.Warnf("cpuinfo count failed: %s", err.Error())

		return nil, false
	}

	res.user = (((info1.User - info2.User) / duration) * 100) / float64(numCPU)
	res.system = (((info1.System - info2.System) / duration) * 100) / float64(numCPU)
	res.iowait = (((info1.Iowait - info2.Iowait) / duration) * 100) / float64(numCPU)
	res.steal = (((info1.Steal - info2.Steal) / duration) * 100) / float64(numCPU)
	res.guest = (((info1.Guest - info2.Guest) / duration) * 100) / float64(numCPU)
	res.idle = (((info1.Idle - info2.Idle) / duration) * 100) / float64(numCPU)
	res.total = (res.user + res.system + res.iowait)

	return res, true
}
