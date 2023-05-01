package snclient

import (
	"fmt"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v3/host"
)

func init() {
	AvailableChecks["check_uptime"] = CheckEntry{"check_uptime", new(CheckUptime)}
}

type CheckUptime struct {
	noCopy noCopy
	data   CheckData
}

/* check_uptime
 * Description: Checks the uptime of the host.
 * Thresholds: uptime
 * Units: s
 */
func (l *CheckUptime) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(0)
	ParseArgs(args, &l.data)

	// collect time metrics (boot + now)
	bootTime, _ := host.BootTime()
	now := time.Now()

	uptime := now.Sub(time.Unix(int64(bootTime), 0))

	mdata := map[string]string{
		"uptime": strconv.FormatInt(int64(uptime.Seconds()), 10),
	}

	// compare ram metrics to thresholds
	if CompareMetrics(mdata, l.data.warnThreshold) {
		state = CheckExitWarning
	}

	if CompareMetrics(mdata, l.data.critThreshold) {
		state = CheckExitCritical
	}

	var days string
	day := int(uptime.Hours() / 24)
	hours := int(uptime.Hours()) - day*24
	minutes := int(uptime.Minutes()) - (hours*60 + day*24*60)
	if day > 7 {
		days = fmt.Sprintf("%vw %vd", day/7, day-day/7*7)
	} else {
		days = fmt.Sprintf("%vd", day)
	}

	bootTimeF := time.Unix(int64(bootTime), 0).Format("2006-01-02 15:04:05")

	output := fmt.Sprintf("uptime: %v %v:%vh, boot: %v (UTC)", days, hours, minutes, bootTimeF)

	return &CheckResult{
		State:  state,
		Output: output,
		Metrics: []*CheckMetric{
			{
				Name:  "uptime",
				Unit:  "s",
				Value: float64(int(uptime.Seconds())),
			},
		},
	}, nil
}
