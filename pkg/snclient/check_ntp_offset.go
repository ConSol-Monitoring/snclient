package snclient

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"time"

	"pkg/convert"
	"pkg/utils"

	"github.com/beevik/ntp"
)

func init() {
	AvailableChecks["check_ntp_offset"] = CheckEntry{"check_ntp_offset", NewCheckNTPOffset}
}

const ntpCMDTimeout = 30

type CheckNTPOffset struct {
	snc       *Agent
	ntpserver []string
	source    string
}

func NewCheckNTPOffset() CheckHandler {
	return &CheckNTPOffset{
		source: "auto",
	}
}

func (l *CheckNTPOffset) Build() *CheckData {
	return &CheckData{
		name:         "check_ntp_offset",
		description:  "Checks the ntp offset.",
		implemented:  ALL,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"server": {value: &l.ntpserver, description: "Fetch offset from this ntp server(s). First valid response is used."},
			"source": {value: &l.source, isFilter: true, description: "Set source of time data instead of auto detect. Can be timedatectl, ntpq, osx or w32tm"},
		},
		defaultFilter:   "none",
		defaultWarning:  "offset > 50 || offset < -50",
		defaultCritical: "offset > 100 || offset < -100",
		detailSyntax:    "offset ${offset_seconds:duration} from ${server}",
		topSyntax:       "${status}: ${list}",
		emptyState:      CheckExitUnknown,
		emptySyntax:     "${status}: could not get any ntp data",
		attributes: []CheckAttribute{
			{name: "source", description: "source of the ntp metrics"},
			{name: "server", description: "ntp server name"},
			{name: "stratum", description: "stratum value (distance to root ntp server)"},
			{name: "jitter", description: "jitter of the clock in milliseconds"},
			{name: "offset", description: "time offset to ntp server in milliseconds"},
			{name: "offset_seconds", description: "time offset to ntp server in seconds"},
		},
		exampleDefault: `
    check_ntp_offset
    OK: offset 2.1ms from 1.2.3.4 (debian.pool.ntp.org) |...
	`,
		exampleArgs: `'warn=offset > 50 || offset < -50' 'crit=offset > 100 || offset < -100'`,
	}
}

func (l *CheckNTPOffset) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	l.snc = snc

	err := l.addSources(ctx, check)
	if err != nil {
		return nil, err
	}

	return check.Finalize()
}

func (l *CheckNTPOffset) addSources(ctx context.Context, check *CheckData) (err error) {
	if len(l.ntpserver) > 0 {
		err = l.addNTPServer(ctx, check)
		if err != nil {
			log.Debugf("failed: ntp: %s", err.Error())

			return err
		}

		return nil
	}

	if l.source == "auto" || l.source == "timedatectl" {
		err = l.addTimeDateCtl(ctx, check, l.source == "timedatectl")
		if err != nil {
			log.Debugf("failed: timedatectl: %s", err.Error())
			if l.source != "auto" {
				return err
			}
		} else {
			return nil
		}
	}

	if l.source == "auto" || l.source == "ntpq" {
		err = l.addNTPQ(ctx, check, l.source == "ntpq")
		if err != nil {
			log.Debugf("failed: ntpq: %s", err.Error())
			if l.source != "auto" {
				return err
			}
		} else {
			return nil
		}
	}

	if l.source == "auto" || l.source == "w32tm" {
		err = l.addW32TM(ctx, check, l.source == "w32tm")
		if err != nil {
			log.Debugf("failed: w32tm: %s", err.Error())
			if l.source != "auto" {
				return err
			}
		} else {
			return nil
		}
	}

	if l.source == "auto" || l.source == "osx" {
		err = l.addOSX(ctx, check, l.source == "osx")
		if err != nil {
			log.Debugf("failed: osx: %s", err.Error())
			if l.source != "auto" {
				return err
			}
		} else {
			return nil
		}
	}

	return err
}

// get offset from systemd timedatectl
func (l *CheckNTPOffset) addTimeDateCtl(ctx context.Context, check *CheckData, force bool) error {
	if !force && runtime.GOOS != "linux" {
		return fmt.Errorf("timedatectl is a linux command")
	}
	output, stderr, rc, _, err := l.snc.runExternalCommandString(ctx, "timedatectl timesync-status", ntpCMDTimeout)
	if err != nil {
		return fmt.Errorf("timedatectl failed: %s\n%s", err.Error(), stderr)
	}
	if rc != 0 {
		return fmt.Errorf("timedatectl failed: %s\n%s", output, stderr)
	}
	entry := l.defaultEntry("timedatectl")

	valid := false
	for _, line := range strings.Split(output, "\n") {
		cols := utils.FieldsN(line, 2)
		if len(cols) < 2 {
			continue
		}
		switch cols[0] {
		case "Server:":
			entry["server"] = cols[1]
		case "Offset:":
			value, _ := time.ParseDuration(cols[1])
			entry["offset"] = fmt.Sprintf("%f", float64(value.Nanoseconds())/1e6)
			entry["offset_seconds"] = fmt.Sprintf("%f", convert.Float64(entry["offset"])/1e3)
			valid = true
		case "Jitter:":
			value, _ := time.ParseDuration(cols[1])
			entry["jitter"] = fmt.Sprintf("%f", float64(value.Nanoseconds())/1e6)
		case "Stratum:":
			entry["stratum"] = cols[1]
		}
	}

	if !valid {
		return fmt.Errorf("cannot parse offset from timedatectl: %s\n%s", output, stderr)
	}

	check.listData = append(check.listData, entry)
	l.addMetrics(check, entry)

	return nil
}

// get offset from ntpq command
func (l *CheckNTPOffset) addNTPQ(ctx context.Context, check *CheckData, force bool) error {
	if !force && runtime.GOOS == "windows" {
		return fmt.Errorf("ntpq is not available on windows")
	}
	output, stderr, rc, _, err := l.snc.runExternalCommandString(ctx, "ntpq -p", ntpCMDTimeout)
	if err != nil {
		return fmt.Errorf("ntpq failed: %s\n%s", err.Error(), stderr)
	}
	if rc != 0 {
		return fmt.Errorf("ntpq failed: %s\n%s", output, stderr)
	}
	entry := l.defaultEntry("ntpq")

	valid := false
	unusable := 0
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "*") {
			unusable++

			continue
		}
		cols := strings.Fields(line)
		if len(cols) < 10 {
			continue
		}
		valid = true
		entry["server"] = fmt.Sprintf("%s (%s)", strings.TrimPrefix(cols[0], "*"), cols[1])
		entry["offset"] = strings.TrimSuffix(cols[8], "ms")
		entry["offset_seconds"] = fmt.Sprintf("%f", convert.Float64(entry["offset"])/1e3)
		entry["jitter"] = strings.TrimSuffix(cols[9], "ms")
		entry["stratum"] = cols[2]
	}

	if !valid {
		return fmt.Errorf("ntpq did not return any usable server\n%s", output)
	}

	if !valid {
		return fmt.Errorf("cannot parse offset from ntpq: %s\n%s", output, stderr)
	}

	check.listData = append(check.listData, entry)
	l.addMetrics(check, entry)

	return nil
}

// get offset from windows w32tm.exe
func (l *CheckNTPOffset) addW32TM(ctx context.Context, check *CheckData, force bool) error {
	if !force && runtime.GOOS != "windows" {
		return fmt.Errorf("w32tm.exe is a windows command")
	}
	output, stderr, rc, _, err := l.snc.runExternalCommandString(ctx, "w32tm.exe /query /status /verbose", ntpCMDTimeout)
	if err != nil {
		return fmt.Errorf("w32tm.exe failed: %s\n%s", err.Error(), stderr)
	}
	if rc != 0 {
		return fmt.Errorf("w32tm.exe failed: %s\n%s", output, stderr)
	}
	entry := l.defaultEntry("w32tm")

	valid := false
	for _, line := range strings.Split(output, "\n") {
		cols := utils.TokenizeBy(line, ":", false, false)
		if len(cols) < 2 {
			continue
		}
		cols[1] = strings.TrimSpace(cols[1])
		switch cols[0] {
		case "Source":
			servers := utils.TokenizeBy(cols[1], ",", false, false)
			entry["server"] = servers[0]
		case "Phase Offset":
			value, _ := time.ParseDuration(cols[1])
			entry["offset"] = fmt.Sprintf("%f", float64(value.Nanoseconds())/1e6)
			entry["offset_seconds"] = fmt.Sprintf("%f", convert.Float64(entry["offset"])/1e3)
			valid = true
		case "Stratum":
			stratas := strings.Fields(cols[1])
			entry["stratum"] = stratas[0]
		case "State Machine":
			fields := strings.Fields(cols[1])
			if fields[0] != "2" {
				return fmt.Errorf("w32tm.exe: %s", line)
			}
		}
	}

	if !valid {
		return fmt.Errorf("cannot parse offset from w32tm: %s\n%s", output, stderr)
	}

	check.listData = append(check.listData, entry)
	l.addMetrics(check, entry)

	return nil
}

// get offset on Mac OSX
func (l *CheckNTPOffset) addOSX(ctx context.Context, check *CheckData, force bool) error {
	if !force && runtime.GOOS != "darwin" {
		return fmt.Errorf("this is a mac osx command")
	}

	output, server, err := l.getOSXData(ctx)
	if err != nil {
		return err
	}

	entry := l.defaultEntry("osx")

	reBrackets := regexp.MustCompile(`\((.*)\)\s*$`)

	valid := false
	for _, line := range strings.Split(output, "\n") {
		cols := utils.FieldsN(line, 2)
		if len(cols) < 2 {
			continue
		}
		cols[1] = strings.TrimSpace(cols[1])
		switch cols[0] {
		case "result:":
			dat := strings.Fields(cols[1])
			if dat[0] != "0" {
				return fmt.Errorf("sntp: %s", strings.TrimSpace(line))
			}
		case "addr:":
			entry["server"] = fmt.Sprintf("%s (%s)", server, cols[1])
		case "offset:":
			offsets := reBrackets.FindStringSubmatch(line)
			if len(offsets) >= 2 {
				value, _ := time.ParseDuration(offsets[1] + "s")
				entry["offset"] = fmt.Sprintf("%f", float64(value.Nanoseconds())/1e6)
				entry["offset_seconds"] = fmt.Sprintf("%f", convert.Float64(entry["offset"])/1e3)
				valid = true
			}
		case "stratum:":
			stratas := reBrackets.FindStringSubmatch(line)
			if len(stratas) >= 2 {
				entry["stratum"] = stratas[1]
			}
		}
	}

	if !valid {
		return fmt.Errorf("cannot parse offset from sntp: %s", output)
	}

	check.listData = append(check.listData, entry)
	l.addMetrics(check, entry)

	return nil
}

func (l *CheckNTPOffset) getOSXData(ctx context.Context) (output, server string, err error) {
	// check if ntp is enabled
	output, stderr, exitCode, _, _ := l.snc.runExternalCommandString(ctx, "systemsetup -getusingnetworktime", ntpCMDTimeout)
	if exitCode != 0 {
		log.Debugf("systemsetup -getusingnetworktime: %s\n%s", output, stderr)
	}
	if !strings.Contains(output, "Network Time: On") {
		return "", "", fmt.Errorf("systemsetup -getusingnetworktime: %s", output)
	}

	// get ntp server
	output, stderr, exitCode, _, _ = l.snc.runExternalCommandString(ctx, "systemsetup -getnetworktimeserver", ntpCMDTimeout)
	if exitCode != 0 {
		log.Debugf("systemsetup -getnetworktimeserver: %s\n%s", output, stderr)
	}
	reServers := regexp.MustCompile(`Network Time Server:\s(.*)$`)
	servers := reServers.FindStringSubmatch(output)
	if len(servers) < 2 {
		return "", "", fmt.Errorf("cannot get ntp server from: systemsetup -getnetworktimeserver: %s", output)
	}
	server = servers[1]

	// run sntp
	output, stderr, exitCode, _, _ = l.snc.runExternalCommandString(ctx, fmt.Sprintf("sntp -n 1 -d %s", server), ntpCMDTimeout)
	if exitCode != 0 {
		log.Debugf("failed: sntp %s: %s\n%s", server, output, stderr)
	}

	return output + "\n" + stderr, server, nil
}

// get offset and stratum from user supplied ntp server
func (l *CheckNTPOffset) addNTPServer(_ context.Context, check *CheckData) (err error) {
	options := ntp.QueryOptions{Timeout: time.Duration(ntpCMDTimeout) * time.Second}
	for _, server := range l.ntpserver {
		response, nErr := ntp.QueryWithOptions(server, options)
		if nErr != nil {
			err = nErr
			log.Debugf("ntp query failed %s: %s", server, err.Error())

			continue
		}

		entry := l.defaultEntry("ntp")

		entry["server"] = server
		entry["offset"] = fmt.Sprintf("%f", float64(response.ClockOffset.Nanoseconds())/1e6)
		entry["offset_seconds"] = fmt.Sprintf("%f", convert.Float64(entry["offset"])/1e3)
		entry["stratum"] = fmt.Sprintf("%d", response.Stratum)

		check.listData = append(check.listData, entry)
		l.addMetrics(check, entry)

		return nil
	}

	return
}

func (l *CheckNTPOffset) defaultEntry(source string) map[string]string {
	return map[string]string{
		"source":         source,
		"server":         "",
		"stratum":        "",
		"jitter":         "",
		"offset":         "",
		"offset_seconds": "",
	}
}

func (l *CheckNTPOffset) addMetrics(check *CheckData, entry map[string]string) {
	check.result.Metrics = append(check.result.Metrics,
		&CheckMetric{
			Name:     "offset",
			Unit:     "ms",
			Value:    convert.Float64(entry["offset"]),
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
		},
		&CheckMetric{
			Name:     "stratum",
			Value:    convert.Int64(entry["stratum"]),
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
			Min:      &Zero,
		},
	)
	if entry["jitter"] != "" {
		check.result.Metrics = append(check.result.Metrics,
			&CheckMetric{
				Name:     "jitter",
				Unit:     "ms",
				Value:    convert.Float64(entry["jitter"]),
				Warning:  check.warnThreshold,
				Critical: check.critThreshold,
				Min:      &Zero,
			},
		)
	}
}
