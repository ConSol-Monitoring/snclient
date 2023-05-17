package snclient

import (
	"fmt"
	"time"

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
		topSyntax: "uptime: ${uptime}h, boot: ${boottime}",
	}
	_, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	// collect time metrics (boot + now)
	bootTime, _ := host.BootTime()
	now := time.Now()

	uptime := now.Sub(time.Unix(int64(bootTime), 0))

	check.details["uptime"] = fmt.Sprintf("%.f", uptime.Seconds())

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

	check.details["uptime"] = fmt.Sprintf("%v %v:%v", days, hours, minutes)
	check.details["boottime"] = fmt.Sprintf("%v (UTC)", bootTimeF)

	check.result.Metrics = append(check.result.Metrics, &CheckMetric{
		Name:  "uptime",
		Unit:  "s",
		Value: float64(int(uptime.Seconds())),
	})

	check.Finalize()

	return check.result, nil
}
