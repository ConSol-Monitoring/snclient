package snclient

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"strings"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/utils"
)

func init() {
	AvailableChecks["check_multi"] = CheckEntry{"check_multi", NewCheckMulti}
}

type (
	checkMultiConfigKey  struct{}
	checkMultiDepthKey   struct{}
	checkMultiCounterKey struct{}
)

type checkMultiCounter struct {
	count     int64
	maxChecks int64
}

type CheckMulti struct {
	commands TaggedCommandList
	config   string
}

var checkMultiAttributes = []CheckAttribute{
	{name: "count", description: "Total number of checks executed"},
	{name: "ok_count", description: "Number of checks in OK state"},
	{name: "warning_count", description: "Number of checks in WARNING state"},
	{name: "critical_count", description: "Number of checks in CRITICAL state"},
	{name: "unknown_count", description: "Number of checks in UNKNOWN state"},
	{name: "problem_count", description: "Number of checks in non-OK state"},
	{name: "name", description: "Name/Tag of the check"},
	{name: "tag", description: "Alias for name"},
	{name: "command", description: "Command executed"},
	{name: "state", description: "Exit code of the check (0=OK, 1=WARNING, 2=CRITICAL, 3=UNKNOWN)"},
	{name: "status", description: "Status text of the check (OK, WARNING, CRITICAL, UNKNOWN)"},
	{name: "output", description: "Check output"},
	{name: "shortoutput", description: "First line of the check output"},
}

func NewCheckMulti() CheckHandler {
	return &CheckMulti{
		commands: make(TaggedCommandList, 0),
	}
}

func (l *CheckMulti) Build() *CheckData {
	return &CheckData{
		name: "check_multi",
		description: `Runs multiple checks and aggregates their status, output and performance data.

	By default 'CheckMulti' is enabled, but you can disable it in the '[/modules]' section of the snclient_local.ini.
	You can also set 'max checks' in the '[/settings/check/multi]' section of the snclient_local.ini, which limits
	the number of checks that can be configured.

	When using the inline mode, you can only use available commands (run 'check_index' to get a full list).

	You can also define custom check sections in the config file, for example:
  [/settings/check/multi/mycheck]
  command[alias1] = check_process process=123
  command[alias2] = check_process process=345

  This can be executed with 'check_multi "config=mycheck"'.

	It's also possible to use custom scripts in the config section, for example:
	[/settings/check/multi/myscript]
	command[alias1] = /path/to/plugin1
	command[alias2] = /path/to/plugin2
	command[alias3] = /path/to/plugin3

	This can be executed with 'check_multi "config=myscript"'.
`,
		implemented:   ALL,
		disableFilter: true,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"command": {value: &l.commands, description: "Check command to execute with mandatory unique tag, e.g. command[tag]=..."},
			"config":  {value: &l.config, description: "Config section name under [/settings/check/multi/< section >] to execute"},
		},
		conditionAlias: map[string]map[string]string{
			"warning_count":  {"warn_count": "warning_count"},
			"critical_count": {"crit_count": "critical_count"},
		},
		attributes:      checkMultiAttributes,
		defaultWarning:  "warning_count > 0",
		defaultCritical: "critical_count > 0",
		defaultUnknown:  "unknown_count > 0",
		okSyntax: "{{ if problem_count gt 0 }}%(status) - %(count) plugins checked: " +
			"%(ok_count) ok, %(warning_count) warning, %(critical_count) critical, " +
			"%(unknown_count) unknown - %(problem_list){{ ELSE }}%(status) - " +
			"%(count) plugins checked, %(ok_count) ok{{ END }}",
		topSyntax:    "%(status) - %(count) plugins checked: %(ok_count) ok, %(warning_count) warning, %(critical_count) critical, %(unknown_count) unknown - %(problem_list)",
		detailSyntax: "%(name): %(shortoutput)",
		emptySyntax:  "%(status) - no checks executed",
		emptyState:   CheckExitUnknown,
		exampleDefault: `
    check_multi "command[check_process]=check_process 'process=firefox'" "command[check_memory]=check_memory 'type=physical' 'crit=used_pct gt 80%'"
	OK - 2 plugins checked, 2 ok |'check_process::count'=1;;;0 ... 'check_memory::physical %'=78.7%;;;0;100
	[check_process] OK - all 1 processes are ok.
	[check_memory] OK - physical = 12.59 GiB/16.00 GiB (78.7%)

	You can define 'warning' and 'critical' conditions based on the number of checks in a certain state (see attributes below):

	check_multi "command[check_dummy1]=check_dummy 0 'OK - check works'" "command[check_dummy2]=check_dummy 1 'WARNING - problem found'" "critical=problem_count gt 0"
	CRITICAL - 2 plugins checked: 1 ok, 1 warning, 0 critical, 0 unknown - warning(check_dummy2: WARNING - problem found)
	[check_dummy1] OK - check works
	[check_dummy2] WARNING - problem found

	You can also override the 'top-syntax' and use IF ELSE statements to get a certain output based on the results:

	check_multi "command[check_dummy1]=check_dummy 0 'OK'" "command[check_dummy2]=check_dummy 2 'CRITICAL'" \
				"top-syntax={{ if ok_count gt 0 }}OK - %(ok_count)/%(count) checks are OK {{ ELSE }}CRITICAL - all checks failed{{ END }}"
	OK - 1/2 checks are OK
	[check_dummy1] OK
	[check_dummy2] CRITICAL
	`,
	}
}

type multiChildCheck struct {
	tag      string
	cmdStr   string
	isInline bool
}

func (l *CheckMulti) childTimeoutResult(timeout time.Duration, totalTimeout int64) *CheckResult {
	return &CheckResult{
		State:  CheckExitUnknown,
		Output: fmt.Sprintf("timed out after %s (reached check_multi timeout of %ds)", l.formatTimeout(timeout), totalTimeout),
	}
}

func (l *CheckMulti) overallTimeoutResult(check *CheckData, snc *Agent, children []childRecord) *CheckResult {
	timeout := snc.getBuiltinCmdTimeout()
	details := make([]string, 0, len(children))
	for _, child := range children {
		details = append(details, check.result.LiteralizeDetails(l.timeoutDetail(child)))
	}

	check.result.State = CheckExitUnknown
	check.result.Output = fmt.Sprintf("UNKNOWN - check_multi timed out after %ds", timeout)
	check.result.Details = strings.Join(details, "\n")

	return check.result
}

func (l *CheckMulti) externalScriptTimeoutResult(res *CheckResult) bool {
	return res.State == CheckExitUnknown && strings.Contains(res.Output, "script run into timeout after")
}

func (l *CheckMulti) externalScriptTimeout(snc *Agent) int64 {
	timeout, ok, err := snc.config.Section("/settings/external scripts").GetDuration("timeout")
	if err != nil || !ok || timeout <= 0 {
		return snc.getBuiltinCmdTimeout()
	}

	return int64(math.Ceil(timeout))
}

func (l *CheckMulti) formatDuration(d time.Duration) string {
	sec := d.Seconds()
	if sec < 0 {
		sec = 0
	}
	if sec >= 1.0 && math.Abs(sec-math.Round(sec)) < 0.05 {
		return fmt.Sprintf("%ds", int64(math.Round(sec)))
	}

	return fmt.Sprintf("%.1fs", sec)
}

func (l *CheckMulti) formatTimeout(d time.Duration) string {
	seconds := max(int64(math.Ceil(d.Seconds())), 0)

	return fmt.Sprintf("%ds", seconds)
}

func (l *CheckMulti) remainingTimeout(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0
	}

	remaining := time.Until(deadline)
	if remaining < 0 {
		return 0
	}

	return remaining
}

func (l *CheckMulti) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	enabled, _, _ := snc.config.Section("/modules").GetBool("CheckMulti")
	if !enabled {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: "module CheckMulti is not enabled in /modules section",
		}, nil
	}
	timeout := snc.getBuiltinCmdTimeout()
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	ctx = timeoutCtx

	depth, _ := ctx.Value(checkMultiDepthKey{}).(int)
	if depth > 5 {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: "recursion limit exceeded for check_multi",
		}, nil
	}
	ctx = context.WithValue(ctx, checkMultiDepthKey{}, depth+1)

	maxChecks, ok, err := snc.config.Section("/settings/check/multi").GetInt("max checks")
	if err != nil || !ok || maxChecks <= 0 {
		maxChecks = 20
	}

	if _, ok := ctx.Value(checkMultiCounterKey{}).(*checkMultiCounter); !ok {
		counter := &checkMultiCounter{maxChecks: maxChecks}
		ctx = context.WithValue(ctx, checkMultiCounterKey{}, counter)
	}

	activeConfigs, _ := ctx.Value(checkMultiConfigKey{}).(map[string]bool)
	if activeConfigs == nil {
		activeConfigs = make(map[string]bool)
	}

	if l.config != "" {
		if activeConfigs[l.config] {
			return &CheckResult{
				State:  CheckExitUnknown,
				Output: fmt.Sprintf("loop detected: check_multi config %s is already running in the call chain", l.config),
			}, nil
		}
		newActive := make(map[string]bool, len(activeConfigs)+1)
		maps.Copy(newActive, activeConfigs)
		newActive[l.config] = true
		ctx = context.WithValue(ctx, checkMultiConfigKey{}, newActive)
	}

	childChecks, res := l.buildChildChecks(snc)
	if res != nil {
		return res, nil
	}

	if len(childChecks) == 0 {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: "no checks or config specified",
		}, nil
	}

	return l.executeChildChecks(ctx, snc, check, childChecks)
}

// buildChildChecks assembles the list of child checks from config section and inline args.
func (l *CheckMulti) buildChildChecks(snc *Agent) ([]multiChildCheck, *CheckResult) {
	childChecks := []multiChildCheck{}
	seenTags := make(map[string]bool)

	if l.config != "" {
		configChecks, res := l.buildConfigChecks(snc)
		if res != nil {
			return nil, res
		}
		for _, chk := range configChecks {
			if seenTags[chk.tag] {
				return nil, &CheckResult{
					State:  CheckExitUnknown,
					Output: fmt.Sprintf("duplicate command tag: %s", chk.tag),
				}
			}
			seenTags[chk.tag] = true
			childChecks = append(childChecks, chk)
		}
	}

	for _, cmd := range l.commands {
		if seenTags[cmd.Tag] {
			return nil, &CheckResult{
				State:  CheckExitUnknown,
				Output: fmt.Sprintf("duplicate command tag: %s", cmd.Tag),
			}
		}
		seenTags[cmd.Tag] = true
		childChecks = append(childChecks, multiChildCheck{
			tag:      cmd.Tag,
			cmdStr:   cmd.Command,
			isInline: true,
		})
	}

	return childChecks, nil
}

// buildConfigChecks loads checks from the named config section.
func (l *CheckMulti) buildConfigChecks(snc *Agent) ([]multiChildCheck, *CheckResult) {
	secName := "/settings/check/multi/" + l.config
	sec, ok := snc.config.sections[secName]

	if !ok || len(sec.keys) == 0 {
		return nil, &CheckResult{
			State:  CheckExitUnknown,
			Output: fmt.Sprintf("no checks defined in config section %s", secName),
		}
	}

	childChecks := make([]multiChildCheck, 0, len(sec.keys))

	for _, key := range sec.keys {
		rawVal := sec.data[key]
		if !strings.HasPrefix(key, "command[") || !strings.HasSuffix(key, "]") {
			return nil, &CheckResult{
				State:  CheckExitUnknown,
				Output: fmt.Sprintf("invalid check_multi config entry: %s (must be in format command[tag]=<command>)", key),
			}
		}
		tag := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(key, "command["), "]"))
		if strings.ContainsAny(tag, DefaultNastyCharacters+"=") {
			return nil, &CheckResult{
				State:  CheckExitUnknown,
				Output: fmt.Sprintf("command tag contains invalid characters: %s", tag),
			}
		}
		if strings.TrimSpace(tag) == "" {
			return nil, &CheckResult{
				State:  CheckExitUnknown,
				Output: "empty command tag in config section",
			}
		}
		if strings.TrimSpace(rawVal) == "" {
			return nil, &CheckResult{
				State:  CheckExitUnknown,
				Output: fmt.Sprintf("empty command for tag %s in config section", tag),
			}
		}
		childChecks = append(childChecks, multiChildCheck{
			tag:      tag,
			cmdStr:   rawVal,
			isInline: false,
		})
	}

	return childChecks, nil
}

type childRecord struct {
	tag              string
	childOutput      string
	durationStr      string
	parentTimedOut   bool
	externalTimedOut bool
}

type childCheckCounts struct {
	count, ok, warning, critical, unknown int64
}

func (counts *childCheckCounts) add(state string) {
	counts.count++
	switch state {
	case "0":
		counts.ok++
	case "1":
		counts.warning++
	case "2":
		counts.critical++
	default:
		counts.unknown++
	}
}

func (l *CheckMulti) recordChildResult(
	check *CheckData,
	childCheck multiChildCheck,
	result *CheckResult,
	record childRecord,
	hasEntryThresholds bool,
	counts *childCheckCounts,
	metrics *[]*CheckMetric,
) {
	firstLine := strings.TrimRight(strings.Split(record.childOutput, "\n")[0], "\r\n ")
	entryState := fmt.Sprintf("%d", result.State)
	entry := map[string]string{
		"name":        record.tag,
		"tag":         record.tag,
		"command":     childCheck.cmdStr,
		"state":       entryState,
		"status":      result.StateString(),
		"shortoutput": firstLine,
		"output":      record.childOutput,
		"_state":      entryState,
		"_skip":       "1",
		"_count":      "1",
	}

	if hasEntryThresholds {
		thresholdEntry := maps.Clone(entry)
		check.Check(thresholdEntry, check.warnThreshold, check.critThreshold, check.unknownThreshold, check.okThreshold)
		check.result.EscalateStatus(convert.Int64(thresholdEntry["_state"]))
	}

	counts.add(entry["_state"])
	check.listData = append(check.listData, entry)
	*metrics = appendChildMetrics(*metrics, result, record.tag)
}

// runOneChild executes a single child check and returns the result along with metadata.
// The parent context deadline takes precedence over an external script timeout.
func (l *CheckMulti) runOneChild(ctx context.Context, snc *Agent, chk multiChildCheck) (*CheckResult, childRecord, bool) {
	childTimeout := l.remainingTimeout(ctx)
	childStart := time.Now()
	res, fatal := l.runChildCheck(ctx, snc, chk)
	childElapsed := time.Since(childStart)
	parentTimedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
	externalTimedOut := l.externalScriptTimeoutResult(res)

	if parentTimedOut {
		res = l.childTimeoutResult(childTimeout, snc.getBuiltinCmdTimeout())
		externalTimedOut = false
	}

	rec := childRecord{
		tag:              chk.tag,
		childOutput:      res.BuildOutputString(),
		durationStr:      l.formatDuration(childElapsed),
		parentTimedOut:   parentTimedOut,
		externalTimedOut: externalTimedOut,
	}

	return res, rec, fatal
}

func (l *CheckMulti) timeoutDetail(child childRecord) string {
	if child.parentTimedOut {
		return fmt.Sprintf("[%s] %s", child.tag, child.childOutput)
	}

	output := strings.TrimRight(child.childOutput, "\r\n ")

	return fmt.Sprintf("[%s] %s (took %s)", child.tag, output, child.durationStr)
}

func (l *CheckMulti) childDetail(child childRecord, externalTimeout int64) string {
	output := child.childOutput
	if child.externalTimedOut {
		firstLine := strings.TrimRight(strings.Split(output, "\n")[0], "\r\n ")
		expectedTimeoutMsg := fmt.Sprintf("timeout after %ds", externalTimeout)
		if strings.Contains(firstLine, expectedTimeoutMsg) {
			output = fmt.Sprintf("%s (reached external scripts timeout of %ds)", firstLine, externalTimeout)
		} else {
			output = firstLine
		}
	}

	return fmt.Sprintf("[%s] %s", child.tag, output)
}

// executeChildChecks runs all child checks and aggregates results.
func (l *CheckMulti) executeChildChecks(ctx context.Context, snc *Agent, check *CheckData, childChecks []multiChildCheck) (*CheckResult, error) {
	var counts childCheckCounts

	executedChildren := make([]childRecord, 0, len(childChecks))
	allMetrics := make([]*CheckMetric, 0)

	hasEntryThresholds := check.HasThreshold("name") || check.HasThreshold("tag") || check.HasThreshold("command") ||
		check.HasThreshold("output") || check.HasThreshold("shortoutput") || check.HasThreshold("status") || check.HasThreshold("state")

	for _, chk := range childChecks {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return l.overallTimeoutResult(check, snc, executedChildren), nil
		}

		if res := l.incrementCheckMultiCounter(ctx); res != nil {
			return res, nil
		}

		res, rec, fatal := l.runOneChild(ctx, snc, chk)
		if fatal {
			return res, nil
		}

		if rec.parentTimedOut {
			executedChildren = append(executedChildren, rec)

			return l.overallTimeoutResult(check, snc, executedChildren), nil
		}

		executedChildren = append(executedChildren, rec)
		l.recordChildResult(check, chk, res, rec, hasEntryThresholds, &counts, &allMetrics)
	}

	detailsList := make([]string, 0, len(executedChildren))
	externalTimeout := l.externalScriptTimeout(snc)
	for _, rec := range executedChildren {
		detailsList = append(detailsList, check.result.LiteralizeDetails(l.childDetail(rec, externalTimeout)))
	}

	problemCount := counts.warning + counts.critical + counts.unknown
	check.details = map[string]string{
		"count":          fmt.Sprintf("%d", counts.count),
		"ok_count":       fmt.Sprintf("%d", counts.ok),
		"warning_count":  fmt.Sprintf("%d", counts.warning),
		"warn_count":     fmt.Sprintf("%d", counts.warning),
		"critical_count": fmt.Sprintf("%d", counts.critical),
		"crit_count":     fmt.Sprintf("%d", counts.critical),
		"unknown_count":  fmt.Sprintf("%d", counts.unknown),
		"problem_count":  fmt.Sprintf("%d", problemCount),
	}

	check.result.Metrics = allMetrics
	check.result.Details = strings.Join(detailsList, "\n")

	return check.Finalize()
}

func appendChildMetrics(allMetrics []*CheckMetric, res *CheckResult, tag string) []*CheckMetric {
	for _, m := range res.Metrics {
		metricCopy := *m
		metricCopy.Name = fmt.Sprintf("%s::%s", tag, m.Name)
		metricCopy.SkipStateCheck = true
		allMetrics = append(allMetrics, &metricCopy)
	}

	return allMetrics
}

func (l *CheckMulti) incrementCheckMultiCounter(ctx context.Context) *CheckResult {
	counter, _ := ctx.Value(checkMultiCounterKey{}).(*checkMultiCounter)
	counter.count++
	if counter.count > counter.maxChecks {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: fmt.Sprintf("number of checks (%d) exceeds max checks limit (%d)", counter.count, counter.maxChecks),
		}
	}

	return nil
}

// runChildCheck executes a single child check and returns its result.
// The second return value is true when the error is fatal and the caller should stop processing.
func (l *CheckMulti) runChildCheck(ctx context.Context, snc *Agent, chk multiChildCheck) (*CheckResult, bool) {
	tokens := utils.Tokenize(chk.cmdStr)
	tokens, err := utils.TrimQuotesList(tokens)

	if err != nil || len(tokens) == 0 {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: fmt.Sprintf("failed to parse check command: %s", chk.cmdStr),
		}, true
	}

	cmdName := tokens[0]
	cmdArgs := tokens[1:]

	_, isKnown := snc.getCheck(cmdName, false)

	if chk.isInline && !isKnown {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: fmt.Sprintf("unknown check command: %s (inline checks only support existing check commands)", cmdName),
		}, true
	}

	if isKnown {
		return snc.RunCheckWithContext(ctx, cmdName, cmdArgs, 0, nil, false), false
	}

	timeout := snc.getBuiltinCmdTimeout()
	if configuredTimeout, ok, err := snc.config.Section("/settings/external scripts").GetInt("timeout"); err == nil && ok && configuredTimeout > 0 {
		timeout = configuredTimeout
	}
	stdout, stderr, exitCode, _ := snc.runExternalCheckString(ctx, chk.cmdStr, timeout)
	out := stdout
	if stderr != "" && !strings.Contains(out, stderr) {
		if out != "" {
			out += "\n"
		}
		out += "[" + stderr + "]"
	}
	res := &CheckResult{
		State:  exitCode,
		Output: out,
	}
	res.ParsePerformanceDataFromOutput()

	return res, false
}
