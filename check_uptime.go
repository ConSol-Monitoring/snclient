package snclient

import (
	"fmt"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v3/host"
)

func init() {
	AvailableChecks["check_uptime"] = CheckEntry{"check_uptime", new(check_uptime)}
}

type check_uptime struct {
	noCopy noCopy
}

/* check_uptime todo
 * todo
 */
func (l *check_uptime) Check(args []string) (*CheckResult, error) {

	// default state: OK
	state := int64(0)
	argList := ParseArgs(args)
	var warnTreshold Treshold
	var critTreshold Treshold

	// parse treshold args
	for _, arg := range argList {
		switch arg.key {
		case "warn", "warning":
			warnTreshold = ParseTreshold(arg.value)
		case "crit", "critical":
			critTreshold = ParseTreshold(arg.value)
		}
	}

	// collect time metrics (boot + now)
	bootTime, _ := host.BootTime()
	now := time.Now()

	uptime := now.Sub(time.Unix(int64(bootTime), 0))

	mdata := []MetricData{MetricData{name: "uptime", value: strconv.FormatInt(int64(uptime.Seconds()), 10)}}

	// compare ram metrics to tresholds
	if CompareMetrics(mdata, warnTreshold) {
		state = CheckExitWarning
	}

	if CompareMetrics(mdata, critTreshold) {
		state = CheckExitCritical
	}

	day := int(uptime.Hours() / 24)
	days := ""
	hours := int(int(uptime.Hours()) - day*24)
	minutes := int(int(uptime.Minutes()) - hours*60)
	if day > 7 {
		days = fmt.Sprintf("%vw %vd", int(day/7), day-int(day/7))
	} else {
		days = fmt.Sprintf("%vd", day)
	}

	output := fmt.Sprintf("uptime: %v %v:%vh, boot: %v (UTC)", days, hours, minutes, time.Unix(int64(bootTime), 0).Format("2006-01-02 15:04:05"))

	min, _ := strconv.ParseFloat(warnTreshold.value, 64)
	max, _ := strconv.ParseFloat(critTreshold.value, 64)

	return &CheckResult{
		State:  state,
		Output: output,
		Metrics: []*CheckMetric{
			&CheckMetric{
				Name:  "uptime",
				Unit:  "s",
				Value: float64(int(uptime.Seconds())),
				Min:   &min,
				Max:   &max,
			},
		},
	}, nil
}
