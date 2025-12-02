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

//nolint:funlen // moving these statements to new helper functions would be illogical
func (l *CheckCPUUtilization) getMetrics(scanLookBack uint64) (res *CPUUtilizationResult, ok bool) {
	res = &CPUUtilizationResult{}

	counter := l.snc.Counter.Get("cpuinfo", "info")
	if counter == nil {
		return nil, false
	}

	scanLookBack64, err := convert.Int64E(scanLookBack)
	if err != nil {
		log.Warnf("failed to convert scan look back to int64: %s", err.Error())

		return nil, false
	}

	if err = counter.CheckRetention(time.Second*time.Duration(scanLookBack64), 0); err != nil {
		log.Tracef("cpuinfo counter cant hold the query range: %s", err.Error())
	}

	if err = counter.CheckRetention(time.Second*time.Duration(scanLookBack64), 100); err != nil {
		log.Warnf("cpuinfo counter cant hold the query range even when extended: %s", err.Error())

		return nil, false
	}

	cpuinfoLatest := counter.GetLast()
	cpuinfoOldest := counter.GetFirst()
	if cpuinfoLatest == nil {
		log.Warnf("latest cpuinfo value seems to be null. counter might not be populated yet.")

		return nil, false
	}
	if cpuinfoOldest == nil {
		log.Warnf("oldest cpuinfo value seems to be null. counter might not be populated yet.")

		return nil, false
	}

	cpuinfoCounterDuration := cpuinfoLatest.UnixMilli - cpuinfoOldest.UnixMilli
	if cpuinfoCounterDuration < scanLookBack64*1000 {
		log.Tracef("cpuinfo counter has %d ms between its latest and oldest value, cannot properly provide %d s range of query", cpuinfoCounterDuration, scanLookBack)

		// Optionally we can wait on this thread while other threads fill the counter up.
	}

	cpuinfoLookBackAgo := counter.GetAt(time.Now().Add(-time.Duration(scanLookBack64) * time.Second))
	if cpuinfoLookBackAgo == nil {
		log.Warnf("cpuinfo value search with lower bound of now-%d seconds returned null", scanLookBack)

		return nil, false
	}

	duration := float64(cpuinfoLatest.UnixMilli - cpuinfoLookBackAgo.UnixMilli)
	acceptableDurationMultipler := 0.5
	minimumAcceptableDuration := float64(scanLookBack) * 1000 * acceptableDurationMultipler

	switch {
	case duration <= 0:
		// This case might happen if there is only one counter value so far
		log.Tracef("counter query from now-%d seconds <-> latest returned a range of %f ms. This is not positive, there might not be enough values recorded yet.", scanLookBack, duration)

		return nil, false
	case duration < minimumAcceptableDuration:
		log.Tracef("counter query from now-%d seconds <-> latest returned a range of %f ms. This is not bellow the acceptable range, the data may be unrepresentative. "+
			"acceptableDurationMultipler * scanLookBack seconds : %f * %f = %f ",
			scanLookBack, duration, acceptableDurationMultipler, float64(scanLookBack), minimumAcceptableDuration)

		// Optionally we can return an empty result here
	case duration <= float64(scanLookBack)*1000:
		log.Tracef("counter query from now-%d seconds <-> latest returned a range of %f ms. This is in the acceptable range, the data should be representative. "+
			"acceptableDurationMultipler * scanLookBack seconds : %f * %f = %f ",
			scanLookBack, duration, acceptableDurationMultipler, float64(scanLookBack), minimumAcceptableDuration)
	default:
		log.Tracef("counter query from now-%d seconds <-> latest returned a range of %f ms. This is higher than the query range and something must have gone wrong.", scanLookBack, duration)
	}

	info1, ok := cpuinfoLatest.Value.(*cpuinfo.TimesStat)
	if !ok {
		return nil, false
	}
	info2, ok := cpuinfoLookBackAgo.Value.(*cpuinfo.TimesStat)
	if !ok {
		return nil, false
	}

	numCPU, err := cpuinfo.Counts(true)
	if err != nil {
		log.Warnf("cpuinfo count failed: %s", err.Error())

		return nil, false
	}

	durationInS := duration / 1e3 // cpu times are measured in seconds
	res.user = (((info1.User - info2.User) / durationInS * 100) / float64(numCPU))
	res.system = (((info1.System - info2.System) / durationInS * 100) / float64(numCPU))
	res.iowait = (((info1.Iowait - info2.Iowait) / durationInS * 100) / float64(numCPU))
	res.steal = (((info1.Steal - info2.Steal) / durationInS * 100) / float64(numCPU))
	res.guest = (((info1.Guest - info2.Guest) / durationInS * 100) / float64(numCPU))
	res.idle = (((info1.Idle - info2.Idle) / durationInS * 100) / float64(numCPU))
	res.total = (res.user + res.system + res.iowait)

	return res, true
}
