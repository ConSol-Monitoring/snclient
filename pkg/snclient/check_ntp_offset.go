package snclient

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/beevik/ntp"
	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/utils"
)

func init() {
	AvailableChecks["check_ntp_offset"] = CheckEntry{"check_ntp_offset", NewCheckNTPOffset}
}

// according to man 8 ntpq (tally codes section), + * and # and good prefixes
var ntpqPeerOK = regexp.MustCompile(`^[+*#]`)

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
			"source": {value: &l.source, isFilter: true, description: "Set source of time data instead of auto detect. Can be timedatectl, ntpq, chronyc, osx or w32tm"},
		},
		defaultFilter:   "none",
		defaultWarning:  "offset > 50 || offset < -50",
		defaultCritical: "offset > 100 || offset < -100",
		detailSyntax:    "offset ${offset_seconds:duration} from ${server}",
		topSyntax:       "%(status) - ${list}",
		emptyState:      CheckExitUnknown,
		emptySyntax:     "%(status) - could not get any ntp data",
		attributes: []CheckAttribute{
			{name: "source", description: "source of the ntp metrics"},
			{name: "server", description: "ntp server name"},
			{name: "stratum", description: "stratum value (distance to root ntp server)"},
			{name: "jitter", description: "jitter of the clock in milliseconds"},
			{name: "offset", description: "time offset to ntp server in milliseconds"},
			{name: "offset_seconds", description: "time offset to ntp server in seconds", unit: UDuration},
		},
		exampleDefault: `
    check_ntp_offset
    OK - offset 2.1ms from 1.2.3.4 (debian.pool.ntp.org) |...
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

	if l.source == "auto" || l.source == "chronyc" {
		err = l.addChronyc(ctx, check, l.source == "chronyc")
		if err != nil {
			log.Debugf("failed: chronyc: %s", err.Error())
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
	output, stderr, rc, err := l.snc.execCommand(ctx, "timedatectl timesync-status", DefaultCmdTimeout)
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
		entry["_error"] = fmt.Sprintf("cannot parse offset from timedatectl: %s\n%s", output, stderr)
	}

	check.listData = append(check.listData, entry)
	l.addMetrics(check, entry)

	return nil
}

// get offset from chronyc
func (l *CheckNTPOffset) addChronyc(ctx context.Context, check *CheckData, force bool) error {
	if !force && runtime.GOOS != "linux" {
		return fmt.Errorf("chronyc is a linux command")
	}
	output, stderr, rc, err := l.snc.execCommand(ctx, "chronyc tracking", DefaultCmdTimeout)
	if err != nil {
		return fmt.Errorf("chronyc failed: %s\n%s", err.Error(), stderr)
	}
	if rc != 0 {
		return fmt.Errorf("chronyc failed: %s\n%s", output, stderr)
	}
	entry := l.defaultEntry("chronyc")

	reBrackets := regexp.MustCompile(`\((.*)\)\s*$`)
	valid := false
	for _, line := range strings.Split(output, "\n") {
		cols := utils.TokenizeBy(line, ":", false, false)
		if len(cols) < 2 {
			continue
		}
		cols[0] = strings.TrimSpace(cols[0])
		cols[1] = strings.TrimSpace(cols[1])
		switch cols[0] {
		case "Reference ID":
			servers := reBrackets.FindStringSubmatch(line)
			if len(servers) >= 2 {
				entry["server"] = servers[1]
			}
		case "Last offset":
			// make value parsable
			cols[1] = strings.TrimPrefix(cols[1], "+")
			cols[1] = strings.ReplaceAll(cols[1], " seconds", "s")
			value, _ := time.ParseDuration(cols[1])
			entry["offset"] = fmt.Sprintf("%f", float64(value.Nanoseconds())/1e6)
			entry["offset_seconds"] = fmt.Sprintf("%f", convert.Float64(entry["offset"])/1e3)
			valid = true
		case "Stratum":
			entry["stratum"] = cols[1]
		case "Leap status":
			if cols[1] != "Normal" {
				entry["_error"] = fmt.Sprintf("chronyc: %s : %s", cols[0], cols[1])
				entry["_exit"] = "2"
			}
		}
	}

	if !valid {
		entry["_error"] = fmt.Sprintf("cannot parse offset from chronyc: %s\n%s", output, stderr)
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
	output, stderr, rc, err := l.snc.execCommand(ctx, "ntpq -p", DefaultCmdTimeout)
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
		if !ntpqPeerOK.MatchString(line) {
			unusable++

			continue
		}
		cols := strings.Fields(line)
		if len(cols) < 10 {
			continue
		}
		valid = true
		entry["server"] = fmt.Sprintf("%s (%s)", ntpqPeerOK.ReplaceAllString(cols[0], ""), cols[1])
		entry["offset"] = strings.TrimSuffix(cols[8], "ms")
		entry["offset_seconds"] = fmt.Sprintf("%f", convert.Float64(entry["offset"])/1e3)
		entry["jitter"] = strings.TrimSuffix(cols[9], "ms")
		entry["stratum"] = cols[2]
	}

	if !valid {
		entry["_error"] = fmt.Sprintf("ntpq did not return any usable server\n%s", output)
		entry["_exit"] = "2"
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
	entry := l.defaultEntry("w32tm")
	output, stderr, exitCode, err := l.snc.execCommand(ctx, "w32tm.exe /query /status /verbose", DefaultCmdTimeout)
	if err != nil {
		entry["_error"] = fmt.Sprintf("w32tm.exe failed: %s\n%s", err.Error(), stderr)
		check.listData = append(check.listData, entry)

		return nil
	}
	if exitCode != 0 {
		entry["_error"] = fmt.Sprintf("w32tm.exe failed: %s\n%s", output, stderr)
		check.listData = append(check.listData, entry)

		return nil
	}

	var valid bool
	var source string
	var offset string
	var stratum string
	var errorStr string
	if strings.Contains(output, "Phase Offset") {
		valid, source, offset, stratum, errorStr = l.parseW32English(output)
	} else {
		valid, source, offset, stratum, errorStr = l.parseW32AnyLang(output)
	}

	switch {
	case errorStr != "":
		entry["_error"] = errorStr
		entry["_exit"] = "2"
	case valid:
		entry["server"] = source
		entry["offset"] = offset
		entry["offset_seconds"] = fmt.Sprintf("%f", convert.Float64(offset)/1e3)
		entry["stratum"] = stratum

	default:
		entry["_error"] = fmt.Sprintf("cannot parse offset from w32tm: %s\n%s", output, stderr)
		entry["_exit"] = "2"
	}

	check.listData = append(check.listData, entry)
	l.addMetrics(check, entry)

	return nil
}

func (l *CheckNTPOffset) parseW32English(text string) (valid bool, source, offset, stratum, errorStr string) {
	for _, line := range strings.Split(text, "\n") {
		cols := utils.TokenizeBy(line, ":", false, false)
		if len(cols) < 2 {
			continue
		}
		cols[1] = strings.TrimSpace(cols[1])
		switch cols[0] {
		case "Source":
			servers := utils.TokenizeBy(cols[1], ",", false, false)
			source = servers[0]
		case "Phase Offset":
			value, _ := time.ParseDuration(cols[1])
			offset = fmt.Sprintf("%f", float64(value.Nanoseconds())/1e6)
			valid = true
		case "Stratum":
			stratas := strings.Fields(cols[1])
			stratum = stratas[0]
		case "State Machine":
			fields := strings.Fields(cols[1])
			if fields[0] != "2" {
				errorStr = fmt.Sprintf("w32tm.exe: %s", line)
			}
		}
	}

	return valid, source, offset, stratum, errorStr
}

func (l *CheckNTPOffset) parseW32AnyLang(text string) (valid bool, source, offset, stratum, errorStr string) {
	type attr struct {
		key  string
		val  string
		line string
	}
	attributes := []attr{}
	for _, line := range strings.Split(text, "\n") {
		cols := utils.TokenizeBy(line, ":", false, false)
		if len(cols) < 2 {
			continue
		}
		cols[1] = strings.TrimSpace(cols[1])
		attributes = append(attributes, attr{cols[0], cols[1], line})
	}

	if len(attributes) < 12 {
		return false, "", "", "", ""
	}

	// assume output offsets stay sane across languages
	servers := utils.TokenizeBy(attributes[7].val, ",", false, false)
	source = servers[0]

	phase, _ := time.ParseDuration(attributes[9].val)
	offset = fmt.Sprintf("%f", float64(phase.Nanoseconds())/1e6)
	valid = true

	stratas := strings.Fields(attributes[1].val)
	stratum = stratas[0]

	fields := strings.Fields(attributes[11].val)
	if fields[0] != "2" {
		errorStr = fmt.Sprintf("w32tm.exe: %s", attributes[11].line)
	}

	return valid, source, offset, stratum, errorStr
}

// get offset on Mac OSX
func (l *CheckNTPOffset) addOSX(ctx context.Context, check *CheckData, force bool) error {
	if !force && runtime.GOOS != "darwin" {
		return fmt.Errorf("this is a mac osx command")
	}

	entry := l.defaultEntry("osx")

	reBrackets := regexp.MustCompile(`\((.*)\)\s*$`)
	output, server, err := l.getOSXData(ctx)
	if err != nil {
		entry["_error"] = err.Error()
		entry["_exit"] = "2"
		check.listData = append(check.listData, entry)

		return nil //nolint:nilerr // error is returned indirect
	}

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
				entry["_error"] = fmt.Sprintf("sntp: %s", strings.TrimSpace(line))
				entry["_exit"] = "2"
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
		entry["_error"] = fmt.Sprintf("cannot parse offset from sntp: %s", output)
	}

	check.listData = append(check.listData, entry)
	l.addMetrics(check, entry)

	return nil
}

func (l *CheckNTPOffset) getOSXData(ctx context.Context) (output, server string, err error) {
	// check if ntp is enabled
	output, stderr, exitCode, _ := l.snc.execCommand(ctx, "systemsetup -getusingnetworktime", DefaultCmdTimeout)
	if exitCode != 0 {
		log.Debugf("systemsetup -getusingnetworktime: %s\n%s", output, stderr)
	}
	if !strings.Contains(output, "Network Time: On") {
		return "", "", fmt.Errorf("systemsetup -getusingnetworktime: %s", output)
	}

	// get ntp server
	output, stderr, exitCode, _ = l.snc.execCommand(ctx, "systemsetup -getnetworktimeserver", DefaultCmdTimeout)
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
	output, stderr, exitCode, _ = l.snc.execCommand(ctx, fmt.Sprintf("sntp -n 1 -d %s", server), DefaultCmdTimeout)
	if exitCode != 0 {
		log.Debugf("failed: sntp %s: %s\n%s", server, output, stderr)
	}

	return output + "\n" + stderr, server, nil
}

// get offset and stratum from user supplied ntp server
func (l *CheckNTPOffset) addNTPServer(_ context.Context, check *CheckData) (err error) {
	options := ntp.QueryOptions{Timeout: time.Duration(DefaultCmdTimeout) * time.Second}
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

	return err
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
	if entry["_error"] != "" {
		return
	}
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
