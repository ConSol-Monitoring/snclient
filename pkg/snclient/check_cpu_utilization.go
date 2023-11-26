package snclient

import (
	"context"
	"fmt"
	"time"

	"pkg/utils"

	cpuinfo "github.com/shirou/gopsutil/v3/cpu"
)

func init() {
	AvailableChecks["check_cpu_utilization"] = CheckEntry{"check_cpu_utilization", new(CheckCPUUtilization)}
}

const (
	CPURateDuration = 30 * time.Second
)

type CPUUtilizationResult struct {
	total  float64
	user   float64
	system float64
	iowait float64
	steal  float64
	guest  float64
}

type CheckCPUUtilization struct {
	snc *Agent
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
		defaultWarning:  "total > 90",
		defaultCritical: "total > 95",
		topSyntax:       "${status} - ${list}",
		detailSyntax:    "user: ${user}% - system: ${system}% - iowait: ${iowait}% - steal: ${steal}% - guest: ${guest}%",
		attributes: []CheckAttribute{
			{name: "total", description: "Sum of user,system,iowait,steal and guest in percent"},
			{name: "user", description: "User cpu utilization in percent"},
			{name: "system", description: "System cpu utilization in percent"},
			{name: "iowait", description: "IOWait cpu utilization in percent"},
			{name: "steal", description: "Steal cpu utilization in percent"},
			{name: "guest", description: "Guest cpu utilization in percent"},
		},
		exampleDefault: `
    check_cpu_utilization
    OK: CPU load is ok. |'total 5m'=13%;80;90 'total 1m'=13%;80;90 'total 5s'=13%;80;90
	`,
		exampleArgs: `'warn=total > 90%' 'crit=total > 95'`,
	}
}

func (l *CheckCPUUtilization) Check(_ context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	l.snc = snc
	if len(snc.Counter.Keys("cpu")) == 0 {
		return nil, fmt.Errorf("no cpu counter available, make sure CheckSystem / CheckSystemUnix in /modules config is enabled")
	}

	l.addCPUUtilizationMetrics(check)

	return check.Finalize()
}

func (l *CheckCPUUtilization) addCPUUtilizationMetrics(check *CheckData) {
	entry := map[string]string{
		"total":  "0",
		"user":   "0",
		"system": "0",
		"iowait": "0",
		"steal":  "0",
		"guest":  "0",
	}
	check.listData = append(check.listData, entry)

	cpuMetrics, ok := l.getMetrics()
	if !ok {
		return
	}

	entry["total"] = fmt.Sprintf("%.f", cpuMetrics.total)
	entry["user"] = fmt.Sprintf("%.f", cpuMetrics.user)
	entry["system"] = fmt.Sprintf("%.f", cpuMetrics.system)
	entry["iowait"] = fmt.Sprintf("%.f", cpuMetrics.iowait)
	entry["steal"] = fmt.Sprintf("%.f", cpuMetrics.steal)
	entry["guest"] = fmt.Sprintf("%.f", cpuMetrics.guest)

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
	)
}

func (l *CheckCPUUtilization) getMetrics() (res *CPUUtilizationResult, ok bool) {
	res = &CPUUtilizationResult{}

	counter1 := l.snc.Counter.GetAny("cpuinfo", "info")
	counter2 := l.snc.Counter.GetAny("cpuinfo", "info")
	if counter1 == nil || counter2 == nil {
		return nil, false
	}

	cpuinfo1 := counter1.GetLast()
	cpuinfo2 := counter2.GetAt(time.Now().Add(-CPURateDuration))
	if cpuinfo1 == nil || cpuinfo2 == nil {
		return nil, false
	}

	if cpuinfo1.timestamp.Before(cpuinfo2.timestamp) {
		return nil, false
	}
	duration := float64(cpuinfo1.timestamp.Unix() - cpuinfo2.timestamp.Unix())
	if duration <= 0 {
		return nil, false
	}

	info1, ok := cpuinfo1.value.(*cpuinfo.TimesStat)
	if !ok {
		return nil, false
	}
	info2, ok := cpuinfo2.value.(*cpuinfo.TimesStat)
	if !ok {
		return nil, false
	}

	res.user = ((info1.User - info2.User) / duration) * 100
	res.system = ((info1.System - info2.System) / duration) * 100
	res.iowait = ((info1.Iowait - info2.Iowait) / duration) * 100
	res.steal = ((info1.Steal - info2.Steal) / duration) * 100
	res.guest = ((info1.Guest - info2.Guest) / duration) * 100
	res.total = res.user + res.system + res.iowait + res.steal + res.guest

	return res, true
}
