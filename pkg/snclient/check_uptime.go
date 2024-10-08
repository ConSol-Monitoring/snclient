package snclient

import (
	"context"
	"fmt"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/utils"
	"github.com/shirou/gopsutil/v4/host"
)

func init() {
	AvailableChecks["check_uptime"] = CheckEntry{"check_uptime", NewCheckUptime}
}

type CheckUptime struct{}

func NewCheckUptime() CheckHandler {
	return &CheckUptime{}
}

func (l *CheckUptime) Build() *CheckData {
	return &CheckData{
		name:         "check_uptime",
		description:  "Check computer uptime (time since last reboot).",
		implemented:  ALL,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		defaultWarning:  "uptime < 2d",
		defaultCritical: "uptime < 1d",
		topSyntax:       "%(status) - ${list}",
		detailSyntax:    "uptime: ${uptime}, boot: ${boot} (UTC)",
		attributes: []CheckAttribute{
			{name: "uptime", description: "Human readable time since last boot"},
			{name: "uptime_value", description: "Uptime in seconds"},
			{name: "boot", description: "Human readable date of last boot"},
		},
		exampleDefault: `
    check_uptime
    OK - uptime: 3d 02:30h, boot: 2023-11-17 19:33:46 (UTC) |'uptime'=268241s;172800:;86400:
	`,
		exampleArgs: `'warn=uptime < 180s' 'crit=uptime < 60s'`,
	}
}

func (l *CheckUptime) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	// collect time metrics (boot + now)
	bootTime, err := host.BootTime()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve uptime: %s", err.Error())
	}
	bootSeconds, err := convert.Int64E(bootTime)
	if err != nil {
		return nil, fmt.Errorf("failed to convert uptime seconds: %s", err.Error())
	}
	bootTimeUnix := time.Unix(bootSeconds, 0)
	uptime := time.Since(bootTimeUnix)

	// improve readabilty and truncate to full minutes when boot time is more then 10minutes
	trunc := time.Minute
	if uptime.Seconds() < 600 {
		trunc = time.Second
	}

	check.listData = append(check.listData, map[string]string{
		"uptime":       utils.DurationString(uptime.Truncate(trunc)),
		"uptime_value": fmt.Sprintf("%.1f", uptime.Seconds()),
		"boot":         bootTimeUnix.UTC().Format("2006-01-02 15:04:05"),
	})

	check.result.Metrics = append(check.result.Metrics, &CheckMetric{
		Name:     "uptime",
		Unit:     "s",
		Value:    float64(int(uptime.Seconds())),
		Warning:  check.warnThreshold,
		Critical: check.critThreshold,
	})

	check.details = map[string]string{
		"uptime": fmt.Sprintf("%f", uptime.Seconds()),
	}

	return check.Finalize()
}
