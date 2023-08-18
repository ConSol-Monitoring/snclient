package snclient

import (
	"context"
	"fmt"
	"time"

	"pkg/utils"

	"github.com/shirou/gopsutil/v3/host"
)

func init() {
	AvailableChecks["check_uptime"] = CheckEntry{"check_uptime", new(CheckUptime)}
}

type CheckUptime struct{}

func (l *CheckUptime) Build() *CheckData {
	return &CheckData{
		name:        "check_uptime",
		description: "Check computer uptime (time since last reboot).",
		result: &CheckResult{
			State: CheckExitOK,
		},
		defaultWarning:  "uptime < 2d",
		defaultCritical: "uptime < 1d",
		topSyntax:       "${status}: ${list}",
		detailSyntax:    "uptime: ${uptime}, boot: ${boot} (UTC)",
	}
}

func (l *CheckUptime) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	// collect time metrics (boot + now)
	bootTime, err := host.BootTime()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve uptime: %s", err.Error())
	}
	uptime := time.Since(time.Unix(int64(bootTime), 0))

	check.listData = append(check.listData, map[string]string{
		"uptime":       utils.DurationString(uptime.Truncate(time.Minute)),
		"uptime_value": fmt.Sprintf("%.1f", uptime.Seconds()),
		"boot":         time.Unix(int64(bootTime), 0).UTC().Format("2006-01-02 15:04:05"),
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
