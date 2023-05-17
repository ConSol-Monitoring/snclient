package snclient

import (
	"fmt"
	"time"

	"pkg/utils"

	"github.com/shirou/gopsutil/v3/host"
)

func init() {
	AvailableChecks["check_uptime"] = CheckEntry{"check_uptime", new(CheckUptime)}
}

type CheckUptime struct{}

/* check_uptime
 * Description: Checks the uptime of the host.
 * Thresholds: uptime
 * Units: s
 */
func (l *CheckUptime) Check(_ *Agent, args []string) (*CheckResult, error) {
	check := &CheckData{
		result: &CheckResult{
			State: CheckExitOK,
		},
		defaultWarning:  "uptime < 2d",
		defaultCritical: "uptime < 1d",
		topSyntax:       "${status}: ${list}",
		detailSyntax:    "uptime: ${uptime}, boot: ${boot} (UTC)",
	}
	_, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	// collect time metrics (boot + now)
	bootTime, err := host.BootTime()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve uptime: %s", err.Error())
	}
	uptime := time.Since(time.Unix(int64(bootTime), 0))

	check.listData = append(check.listData, map[string]string{
		"uptime": utils.DurationString(uptime.Truncate(time.Minute)),
		"boot":   time.Unix(int64(bootTime), 0).UTC().Format("2006-01-02 15:04:05"),
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
