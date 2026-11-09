package snclient

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"slices"
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
		description:  "Checks the NTP offset.",
		implemented:  ALL,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"server": {value: &l.ntpserver, description: "Fetch the offset from these NTP servers. Uses the first valid response."},
			"source": {value: &l.source, isFilter: true, description: "Choose the time data source instead of automatic detection. Valid values: auto, timedatectl, ntpq, chronyc, macos, w32time, w32tm."},
		},
		defaultFilter:   "none",
		defaultWarning:  "offset > 50 || offset < -50",
		defaultCritical: "offset > 100 || offset < -100",
		detailSyntax:    "offset %(offset_seconds:duration) from %(server)",
		topSyntax:       "%(status) - %(list)",
		emptyState:      CheckExitUnknown,
		emptySyntax:     "%(status) - could not get any ntp data",
		attributes: []CheckAttribute{
			{name: "source", description: "source of the NTP metrics"},
			{name: "server", description: "NTP server name"},
			{name: "stratum", description: "stratum value (distance to root NTP server)"},
			{name: "jitter", description: "jitter of the clock in milliseconds"},
			{name: "offset", description: "Time offset to the NTP server in milliseconds. SNClient adds this value as a metric."},
			{name: "offset_seconds", description: "Time offset to the NTP server in seconds. SNClient does not add this value as a metric. " +
				"SNClient converts thresholds from 'offset_seconds' to 'offset' without a message.", unit: UDuration},
			{name: "leap_indicator", description: "NTP leap indicator: 0 no warning, 1 last minute has 61 seconds, " +
				"2 last minute has 59 seconds, 3 not synchronized. SNClient sets this value only for the w32time source."},
			{name: "state", description: "Clock discipline state: 0 unset, 1 hold, 2 sync, 3 spike. " +
				"SNClient sets this value only for the w32time source."},
			{name: "state_name", description: "Name of the clock discipline state. w32tm shows this name. " +
				"SNClient sets this value only for the w32time source."},
			{name: "last_sync_result", description: "Result of the last synchronization: 0 success, 1 no data, " +
				"2 stale data, 3 change too big, 4 shutdown. SNClient sets this value only for the w32time source."},
			{name: "last_sync_result_name", description: "Name of the last synchronization result. " +
				"SNClient sets this value only for the w32time source."},
			{name: "root_delay", description: "Round-trip delay to the primary time source in milliseconds. " +
				"SNClient sets this value only for the w32time source."},
			{name: "root_dispersion", description: "Maximum error relative to the primary time source in milliseconds. " +
				"SNClient sets this value only for the w32time source."},
			{name: "last_sync", description: "Time of the last successful synchronization as a Unix timestamp in seconds. " +
				"SNClient sets this value only for the w32time source.", unit: UDate},
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

	// convert thresholds
	// only 'offset' is given as a perf data metric, which is in milliseconds
	// any thresholds using 'offset_seconds' have to be converted into seconds and added as an 'offset' threshold

	for _, warnCond := range check.warnThreshold {
		err := warnCond.RunFuncRecursively(convertOffsetSecondOperandToOffset)
		if err != nil {
			return nil, fmt.Errorf("error converting 'offset_seconds' warning threshold to 'offset' threshold: %s", err.Error())
		}
	}

	for _, critCond := range check.critThreshold {
		err := critCond.RunFuncRecursively(convertOffsetSecondOperandToOffset)
		if err != nil {
			return nil, fmt.Errorf("error converting 'offset_seconds' critical threshold to 'offset' threshold: %s", err.Error())
		}
	}

	err := l.addSources(ctx, check)
	if err != nil {
		return nil, err
	}

	return check.Finalize()
}

func convertOffsetSecondOperandToOffset(condition *Condition) (err error) {
	if condition.keyword == "offset_seconds" {
		valueFloat64, valueFloat64ConversionErr := convert.Float64E(condition.value)
		if valueFloat64ConversionErr != nil {
			return fmt.Errorf("could not convert condition value: %s to float: %s", condition.value, valueFloat64ConversionErr.Error())
		}

		condition.keyword = "offset"
		condition.value = fmt.Sprintf("%f", valueFloat64*1000)

		// do not change condition.original
		// that saves a string of what user gave as the condition
	}

	return nil
}

// ntpSource is a provider of time data, tried in the order of ntpSources
type ntpSource struct {
	names []string
	add   func(context.Context, *CheckData, bool) error
}

// names[0] is the canonical name and is used in the debug log
func (l *CheckNTPOffset) ntpSources() []ntpSource {
	return []ntpSource{
		{names: []string{"timedatectl"}, add: l.addTimeDateCtl},
		{names: []string{"chronyc"}, add: l.addChronyc},
		{names: []string{"ntpq"}, add: l.addNTPQ},
		{names: []string{"w32time"}, add: l.addW32Time},
		{names: []string{"w32tm"}, add: l.addW32TM},
		{names: []string{"macos", "osx"}, add: l.addOSX},
	}
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

	for _, source := range l.ntpSources() {
		if l.source != "auto" && !slices.Contains(source.names, l.source) {
			continue
		}

		err = source.add(ctx, check, l.source != "auto")
		if err == nil {
			return nil
		}

		log.Debugf("failed: %s: %s", source.names[0], err.Error())

		if l.source != "auto" {
			return err
		}
	}

	return err
}

// get offset from systemd timedatectl
func (l *CheckNTPOffset) addTimeDateCtl(ctx context.Context, check *CheckData, force bool) error {
	if !force && runtime.GOOS != "linux" {
		return fmt.Errorf("timedatectl is a linux command")
	}
	output, stderr, rc, err := l.snc.execCommand(ctx, "timedatectl timesync-status", l.snc.getBuiltinCmdTimeout())
	if err != nil {
		return fmt.Errorf("timedatectl failed: %s\n%s", err.Error(), stderr)
	}
	if rc != 0 {
		return fmt.Errorf("timedatectl failed: %s\n%s", output, stderr)
	}
	entry := l.defaultEntry("timedatectl")

	valid := false
	for line := range strings.SplitSeq(output, "\n") {
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
	output, stderr, rc, err := l.snc.execCommand(ctx, "chronyc tracking", l.snc.getBuiltinCmdTimeout())
	if err != nil {
		return fmt.Errorf("chronyc failed: %s\n%s", err.Error(), stderr)
	}
	if rc != 0 {
		return fmt.Errorf("chronyc failed: %s\n%s", output, stderr)
	}
	entry := l.defaultEntry("chronyc")

	reBrackets := regexp.MustCompile(`\((.*)\)\s*$`)
	valid := false
	for line := range strings.SplitSeq(output, "\n") {
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
	output, stderr, rc, err := l.snc.execCommand(ctx, "ntpq -p", l.snc.getBuiltinCmdTimeout())
	if err != nil {
		return fmt.Errorf("ntpq failed: %s\n%s", err.Error(), stderr)
	}
	if rc != 0 {
		return fmt.Errorf("ntpq failed: %s\n%s", output, stderr)
	}
	entry := l.defaultEntry("ntpq")

	valid := false
	unusable := 0
	for line := range strings.SplitSeq(output, "\n") {
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

// w32timeState is the state of the clock discipline algorithm (MS-W32T 2.2.7).
type w32timeState uint32

const (
	w32timeStateUnset w32timeState = iota
	w32timeStateHold
	w32timeStateSync
	w32timeStateSpike
)

var w32timeStateNames = map[w32timeState]string{
	w32timeStateUnset: "Unset",
	w32timeStateHold:  "Hold",
	w32timeStateSync:  "Sync",
	w32timeStateSpike: "Spike",
}

// String returns the state name as printed by w32tm /query /status.
func (state w32timeState) String() string {
	if name, ok := w32timeStateNames[state]; ok {
		return name
	}

	return fmt.Sprintf("Unknown(%d)", uint32(state))
}

// w32timeSyncResult is a TimeSync_ReturnResult code (MS-W32T 3.2.5.1).
type w32timeSyncResult uint32

const (
	w32timeSyncResultSuccess w32timeSyncResult = iota
	w32timeSyncResultNoData
	w32timeSyncResultStaleData
	w32timeSyncResultChangeTooBig
	w32timeSyncResultShutdown
)

var w32timeSyncResultNames = map[w32timeSyncResult]string{
	w32timeSyncResultSuccess:      "Success",
	w32timeSyncResultNoData:       "NoData",
	w32timeSyncResultStaleData:    "StaleData",
	w32timeSyncResultChangeTooBig: "ChangeTooBig",
	w32timeSyncResultShutdown:     "Shutdown",
}

// String returns a readable name for the result of the last synchronization.
func (result w32timeSyncResult) String() string {
	if name, ok := w32timeSyncResultNames[result]; ok {
		return name
	}

	return fmt.Sprintf("Unknown(%d)", uint32(result))
}

// w32timeStatus is the status of the local Windows Time service.
// It is filled by queryW32TimeStatus, a port of the windows_exporter (MIT, Copyright The Prometheus Authors).
// See check_ntp_offset_windows.go for the origin.
type w32timeStatus struct {
	LeapIndicator  uint32
	Stratum        uint32
	RootDelay      time.Duration
	RootDispersion time.Duration
	Source         string
	// PhaseOffset is the time difference to the time source, the same value
	// w32tm /query /status prints as Phase Offset.
	PhaseOffset    time.Duration
	State          w32timeState
	LastSyncResult w32timeSyncResult
	// LastSuccessfulSync is zero when the service has not synchronized yet.
	LastSuccessfulSync time.Time
}

// get offset from the w32time.dll of the windows time service, needs no admin rights.
// The status is read by queryW32TimeStatus, which is implemented in
// check_ntp_offset_windows.go.
func (l *CheckNTPOffset) addW32Time(_ context.Context, check *CheckData, force bool) error {
	if !force && runtime.GOOS != "windows" {
		return fmt.Errorf("w32time.dll is a windows library")
	}

	// On 386 there is no w32time.dll in a 32 bit process and the structure layout
	// differs, so automatic detection has to use w32tm.
	if runtime.GOOS == "windows" && runtime.GOARCH == "386" {
		return fmt.Errorf("w32time is not available on 386")
	}

	status, err := queryW32TimeStatus()
	if err != nil {
		return fmt.Errorf("get windows time status: %w", err)
	}

	entry := l.defaultEntry("w32time")

	fillW32TimeEntry(entry, &status)

	check.listData = append(check.listData, entry)
	l.addMetrics(check, entry)

	return nil
}

func fillW32TimeEntry(entry map[string]string, status *w32timeStatus) {
	// the source can carry a flag suffix like "time.windows.com,0x8", w32tm shows the name only
	entry["server"], _, _ = strings.Cut(status.Source, ",")
	entry["stratum"] = fmt.Sprintf("%d", status.Stratum)
	entry["offset"] = fmt.Sprintf("%f", float64(status.PhaseOffset.Nanoseconds())/1e6)
	entry["offset_seconds"] = fmt.Sprintf("%f", convert.Float64(entry["offset"])/1e3)
	entry["leap_indicator"] = fmt.Sprintf("%d", status.LeapIndicator)
	entry["state"] = fmt.Sprintf("%d", uint32(status.State))
	entry["state_name"] = status.State.String()
	entry["last_sync_result"] = fmt.Sprintf("%d", uint32(status.LastSyncResult))
	entry["last_sync_result_name"] = status.LastSyncResult.String()
	entry["root_delay"] = fmt.Sprintf("%f", float64(status.RootDelay.Nanoseconds())/1e6)
	entry["root_dispersion"] = fmt.Sprintf("%f", float64(status.RootDispersion.Nanoseconds())/1e6)
	if !status.LastSuccessfulSync.IsZero() {
		entry["last_sync"] = fmt.Sprintf("%.3f", float64(status.LastSuccessfulSync.UnixMilli())/1e3)
	}

	if status.State != w32timeStateSync {
		entry["_error"] = fmt.Sprintf("w32time: State Machine: %d (%s)", uint32(status.State), status.State)
		entry["_exit"] = "2"
	}
}

// get offset from windows w32tm.exe
func (l *CheckNTPOffset) addW32TM(ctx context.Context, check *CheckData, force bool) error {
	if !force && runtime.GOOS != "windows" {
		return fmt.Errorf("w32tm.exe is a windows command")
	}
	entry := l.defaultEntry("w32tm")
	output, stderr, exitCode, err := l.snc.execCommand(ctx, "w32tm.exe /query /status /verbose", l.snc.getBuiltinCmdTimeout())
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
	for line := range strings.SplitSeq(text, "\n") {
		key, val, ok := splitW32Line(line)
		if !ok {
			continue
		}
		switch key {
		case "Source":
			servers := utils.TokenizeBy(val, ",", false, false)
			source = servers[0]
		case "Phase Offset":
			value, _ := time.ParseDuration(val)
			offset = fmt.Sprintf("%f", float64(value.Nanoseconds())/1e6)
			valid = true
		case "Stratum":
			stratas := strings.Fields(val)
			stratum = stratas[0]
		case "State Machine":
			fields := strings.Fields(val)
			if fields[0] != "2" {
				errorStr = fmt.Sprintf("w32tm.exe: %s", line)
			}
		}
	}

	return valid, source, offset, stratum, errorStr
}

// splitW32Line splits a "key: value" line on the first colon, ignoring quotes.
func splitW32Line(line string) (key, val string, ok bool) {
	key, val, ok = strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}

	return strings.TrimSpace(key), strings.TrimSpace(val), true
}

func (l *CheckNTPOffset) parseW32AnyLang(text string) (valid bool, source, offset, stratum, errorStr string) {
	type attr struct {
		key  string
		val  string
		line string
	}
	attributes := []attr{}
	for line := range strings.SplitSeq(text, "\n") {
		key, val, ok := splitW32Line(line)
		if !ok {
			continue
		}
		attributes = append(attributes, attr{key, val, line})
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

// get offset on macOS
func (l *CheckNTPOffset) addOSX(ctx context.Context, check *CheckData, force bool) error {
	if !force && runtime.GOOS != "darwin" {
		return fmt.Errorf("this is a macOS command")
	}

	entry := l.defaultEntry("macos")

	reBrackets := regexp.MustCompile(`\((.*)\)\s*$`)
	output, server, err := l.getOSXData(ctx)
	if err != nil {
		entry["_error"] = err.Error()
		entry["_exit"] = "2"
		check.listData = append(check.listData, entry)

		return nil //nolint:nilerr // error is returned indirect
	}

	valid := false
	for line := range strings.SplitSeq(output, "\n") {
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
	output, stderr, exitCode, _ := l.snc.execCommand(ctx, "systemsetup -getusingnetworktime", l.snc.getBuiltinCmdTimeout())
	if exitCode != 0 {
		log.Debugf("systemsetup -getusingnetworktime: %s\n%s", output, stderr)
	}
	if !strings.Contains(output, "Network Time: On") {
		return "", "", fmt.Errorf("systemsetup -getusingnetworktime: %s", output)
	}

	// get ntp server
	output, stderr, exitCode, _ = l.snc.execCommand(ctx, "systemsetup -getnetworktimeserver", l.snc.getBuiltinCmdTimeout())
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
	output, stderr, exitCode, _ = l.snc.execCommand(ctx, fmt.Sprintf("sntp -n 1 -d %s", server), l.snc.getBuiltinCmdTimeout())
	if exitCode != 0 {
		log.Debugf("failed: sntp %s: %s\n%s", server, output, stderr)
	}

	return output + "\n" + stderr, server, nil
}

// get offset and stratum from user supplied ntp server
func (l *CheckNTPOffset) addNTPServer(_ context.Context, check *CheckData) (err error) {
	options := ntp.QueryOptions{Timeout: l.snc.getBuiltinCmdTimeout()}
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
	check.result.Metrics = append(
		check.result.Metrics,
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
		check.result.Metrics = append(
			check.result.Metrics,
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
