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

const (
	// defaultMaxChecks sets the default maximum number of checks
	defaultMaxChecks = 20

	// defaultMaxRecursionDepth sets the default maximum recursion depth
	defaultMaxRecursionDepth = 5
)

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
		implemented: ALL,
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
		attributes:               checkMultiAttributes,
		defaultWarning:           "warning_count > 0",
		defaultCritical:          "critical_count > 0",
		defaultUnknown:           "unknown_count > 0",
		thresholdsOverrideCounts: true,
		okSyntax: "{{ if problem_count gt 0 }}%(status) - %(count) plugins checked: " +
			"%(ok_count) ok, %(warning_count) warning, %(critical_count) critical, " +
			"%(unknown_count) unknown - %(problem_list){{ ELSE }}%(status) - " +
			"%(count) plugins checked, %(ok_count) ok{{ END }}",
		topSyntax:        "%(status) - %(count) plugins checked: %(ok_count) ok, %(warning_count) warning, %(critical_count) critical, %(unknown_count) unknown - %(problem_list)",
		detailSyntax:     "%(name): %(shortoutput)",
		longDetailSyntax: "[%(name)] %(output)",
		emptySyntax:      "%(status) - no checks executed",
		emptyState:       CheckExitUnknown,
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
		exampleArgs: `"command[check_cpu]=check_cpu" "command[check_memory]=check_memory"`,
	}
}

type multiChildCheck struct {
	tag      string
	cmdStr   string
	isInline bool
}

func (l *CheckMulti) childTimeoutResult(timeout, totalTimeout time.Duration) *CheckResult {
	return &CheckResult{
		State:  CheckExitUnknown,
		Output: fmt.Sprintf("timed out after %s (reached check_multi timeout of %s)", l.formatTimeout(timeout), totalTimeout),
	}
}

func (l *CheckMulti) overallTimeoutResult(check *CheckData, snc *Agent, children []childRecord) *CheckResult {
	timeout := snc.getBuiltinCmdTimeout()
	details := make([]string, 0, len(children))
	for i := range children {
		child := &children[i]
		detail, err := l.renderLongDetail(check, child, l.timeoutDetailOutput(child))
		if err != nil {
			check.result.State = CheckExitUnknown
			check.result.Output = fmt.Sprintf("UNKNOWN - %s", err.Error())

			return check.result
		}
		if detail != "" {
			details = append(details, detail)
		}
	}

	check.result.State = CheckExitUnknown
	check.result.Output = fmt.Sprintf("UNKNOWN - check_multi timed out after %s", timeout)
	check.result.Details = strings.Join(details, "\n")

	return check.result
}

func (l *CheckMulti) externalScriptTimeout(snc *Agent) time.Duration {
	timeout, ok, err := snc.config.Section("/settings/external scripts").GetDuration("timeout")
	if err != nil || !ok || timeout <= 0 {
		return snc.getBuiltinCmdTimeout()
	}

	return timeout
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
	start := time.Now()
	enabled, _, _ := snc.config.Section("/modules").GetBool("CheckMulti")
	if !enabled {
		return nil, fmt.Errorf("module CheckMulti is not enabled in /modules section")
	}
	timeout := snc.getBuiltinCmdTimeout()
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ctx = timeoutCtx

	depth, _ := ctx.Value(checkMultiDepthKey{}).(int)
	if depth > defaultMaxRecursionDepth {
		return nil, fmt.Errorf("recursion limit exceeded for check_multi")
	}
	ctx = context.WithValue(ctx, checkMultiDepthKey{}, depth+1)

	maxChecks, ok, err := snc.config.Section("/settings/check/multi").GetInt("max checks")
	if err != nil || !ok || maxChecks <= 0 {
		maxChecks = defaultMaxChecks
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
			return nil, fmt.Errorf("loop detected: check_multi config %s is already running in the call chain", l.config)
		}
		newActive := make(map[string]bool, len(activeConfigs)+1)
		maps.Copy(newActive, activeConfigs)
		newActive[l.config] = true
		ctx = context.WithValue(ctx, checkMultiConfigKey{}, newActive)
	}

	childChecks, err := l.buildChildChecks(snc)
	if err != nil {
		return nil, err
	}

	if len(childChecks) == 0 {
		return nil, fmt.Errorf("no checks or config specified")
	}

	return l.executeChildChecks(ctx, snc, check, childChecks, start)
}

// buildChildChecks assembles the list of child checks from config section and inline args.
func (l *CheckMulti) buildChildChecks(snc *Agent) ([]multiChildCheck, error) {
	childChecks := []multiChildCheck{}
	seenTags := make(map[string]bool)

	if l.config != "" {
		configChecks, err := l.buildConfigChecks(snc)
		if err != nil {
			return nil, err
		}
		for _, chk := range configChecks {
			if seenTags[chk.tag] {
				return nil, fmt.Errorf("duplicate command tag: %s", chk.tag)
			}
			seenTags[chk.tag] = true
			childChecks = append(childChecks, chk)
		}
	}

	for _, cmd := range l.commands {
		if seenTags[cmd.Tag] {
			return nil, fmt.Errorf("duplicate command tag: %s", cmd.Tag)
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
func (l *CheckMulti) buildConfigChecks(snc *Agent) ([]multiChildCheck, error) {
	secName := "/settings/check/multi/" + l.config
	sec, ok := snc.config.sections[secName]

	if !ok || len(sec.keys) == 0 {
		return nil, fmt.Errorf("no checks defined in config section %s", secName)
	}

	childChecks := make([]multiChildCheck, 0, len(sec.keys))

	for _, key := range sec.keys {
		rawVal := sec.data[key]
		if !strings.HasPrefix(key, "command[") || !strings.HasSuffix(key, "]") {
			return nil, fmt.Errorf("invalid check_multi config entry: %s (must be in format command[tag]=<command>)", key)
		}
		tag := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(key, "command["), "]"))
		if strings.ContainsAny(tag, DefaultNastyCharacters+"=") {
			return nil, fmt.Errorf("command tag contains invalid characters: %s", tag)
		}
		if strings.TrimSpace(tag) == "" {
			return nil, fmt.Errorf("empty command tag in config section")
		}
		if strings.TrimSpace(rawVal) == "" {
			return nil, fmt.Errorf("empty command for tag %s in config section", tag)
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
	command          string
	state            string
	status           string
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
	result *CheckResult,
	record *childRecord,
	hasEntryThresholds bool,
	counts *childCheckCounts,
	metrics *[]*CheckMetric,
) bool {
	firstLine := strings.TrimRight(strings.Split(record.childOutput, "\n")[0], "\r\n ")
	entry := map[string]string{
		"name":        record.tag,
		"tag":         record.tag,
		"command":     record.command,
		"state":       record.state,
		"status":      record.status,
		"shortoutput": firstLine,
		"output":      record.childOutput,
		"_state":      record.state,
		"_skip":       "1",
		"_count":      "1",
	}
	if !check.MatchMapCondition(check.filter, entry, false) {
		return false
	}
	*metrics = appendChildMetrics(*metrics, result, record.tag)

	if hasEntryThresholds {
		thresholdEntry := maps.Clone(entry)
		check.Check(thresholdEntry, check.warnThreshold, check.critThreshold, check.unknownThreshold, check.okThreshold)
		check.result.EscalateStatus(convert.Int64(thresholdEntry["_state"]))
	}

	counts.add(entry["_state"])
	check.listData = append(check.listData, entry)

	return true
}

// runOneChild executes a single child check and returns the result along with metadata.
// The parent context deadline takes precedence over an external script timeout.
func (l *CheckMulti) runOneChild(ctx context.Context, snc *Agent, chk multiChildCheck) (*CheckResult, childRecord, error) {
	childTimeout := l.remainingTimeout(ctx)
	childStart := time.Now()
	res, err := l.runChildCheck(ctx, snc, chk)
	if err != nil {
		return nil, childRecord{}, err
	}
	childElapsed := time.Since(childStart)
	parentTimedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
	externalTimedOut := res.IsTimeout

	if parentTimedOut {
		res = l.childTimeoutResult(childTimeout, snc.getBuiltinCmdTimeout())
		externalTimedOut = false
	}

	rec := childRecord{
		tag:              chk.tag,
		command:          chk.cmdStr,
		state:            fmt.Sprintf("%d", res.State),
		status:           res.StateString(),
		childOutput:      res.BuildOutputString(),
		durationStr:      l.formatDuration(childElapsed),
		parentTimedOut:   parentTimedOut,
		externalTimedOut: externalTimedOut,
	}

	return res, rec, nil
}

func (l *CheckMulti) timeoutDetailOutput(child *childRecord) string {
	if child.parentTimedOut {
		return child.childOutput
	}

	output := strings.TrimRight(child.childOutput, "\r\n ")

	return fmt.Sprintf("%s (took %s)", output, child.durationStr)
}

func (l *CheckMulti) childDetailOutput(child *childRecord, externalTimeout time.Duration) string {
	output := child.childOutput
	if child.externalTimedOut {
		firstLine := strings.TrimRight(strings.Split(output, "\n")[0], "\r\n ")
		expectedTimeoutMsg := fmt.Sprintf("timeout after %s", externalTimeout)
		if strings.Contains(firstLine, expectedTimeoutMsg) {
			output = fmt.Sprintf("%s (reached external scripts timeout of %s)", firstLine, externalTimeout)
		} else {
			output = firstLine
		}
	}

	return output
}

func (l *CheckMulti) renderLongDetail(check *CheckData, child *childRecord, output string) (string, error) {
	if check.longDetailSyntax == "" {
		return "", nil
	}

	firstLine := strings.TrimRight(strings.Split(output, "\n")[0], "\r\n ")
	attributes := map[string]string{
		"name":        child.tag,
		"tag":         child.tag,
		"command":     child.command,
		"state":       child.state,
		"status":      child.status,
		"output":      output,
		"shortoutput": firstLine,
	}
	detail, err := ReplaceTemplate(check.longDetailSyntax, check.timezone, attributes)
	if err != nil {
		return "", fmt.Errorf("replacing long-detail-syntax failed: %s", err.Error())
	}
	if detail == "" {
		return "", nil
	}

	return check.result.LiteralizeDetails(detail), nil
}

// executeChildChecks runs all child checks and aggregates results.
func (l *CheckMulti) executeChildChecks(
	ctx context.Context,
	snc *Agent,
	check *CheckData,
	childChecks []multiChildCheck,
	start time.Time,
) (*CheckResult, error) {
	var counts childCheckCounts

	executedChildren := make([]childRecord, 0, len(childChecks))
	visibleChildren := make([]childRecord, 0, len(childChecks))
	allMetrics := make([]*CheckMetric, 0, 7)

	hasEntryThresholds := check.HasThreshold("name") || check.HasThreshold("tag") || check.HasThreshold("command") ||
		check.HasThreshold("output") || check.HasThreshold("shortoutput") || check.HasThreshold("status") || check.HasThreshold("state")

	for _, chk := range childChecks {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return l.overallTimeoutResult(check, snc, executedChildren), nil
		}

		if err := l.incrementCheckMultiCounter(ctx); err != nil {
			return nil, err
		}

		res, rec, err := l.runOneChild(ctx, snc, chk)
		if err != nil {
			return nil, err
		}

		if rec.parentTimedOut {
			executedChildren = append(executedChildren, rec)

			return l.overallTimeoutResult(check, snc, executedChildren), nil
		}

		executedChildren = append(executedChildren, rec)
		if l.recordChildResult(check, res, &rec, hasEntryThresholds, &counts, &allMetrics) {
			visibleChildren = append(visibleChildren, rec)
		}
	}

	detailsList := make([]string, 0, len(visibleChildren))
	externalTimeout := l.externalScriptTimeout(snc)
	for i := range visibleChildren {
		rec := &visibleChildren[i]
		detail, err := l.renderLongDetail(check, rec, l.childDetailOutput(rec, externalTimeout))
		if err != nil {
			return nil, err
		}
		if detail != "" {
			detailsList = append(detailsList, detail)
		}
	}

	problemCount := counts.warning + counts.critical + counts.unknown
	timeoutSeconds := snc.getBuiltinCmdTimeout().Seconds()
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

	check.result.Metrics = append(allMetrics,
		&CheckMetric{Name: "ok_count", Value: counts.ok, Min: &Zero, SkipStateCheck: true},
		&CheckMetric{Name: "warning_count", Value: counts.warning, Min: &Zero, SkipStateCheck: true},
		&CheckMetric{Name: "critical_count", Value: counts.critical, Min: &Zero, SkipStateCheck: true},
		&CheckMetric{Name: "unknown_count", Value: counts.unknown, Min: &Zero, SkipStateCheck: true},
		&CheckMetric{Name: "problem_count", Value: problemCount, Min: &Zero, SkipStateCheck: true},
		&CheckMetric{Name: "total_count", Value: counts.count, Min: &Zero, SkipStateCheck: true},
		&CheckMetric{Name: "time", Value: utils.ToPrecision(time.Since(start).Seconds(), 2), Unit: "s", Min: &Zero, Max: &timeoutSeconds, SkipStateCheck: true},
	)
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

func (l *CheckMulti) incrementCheckMultiCounter(ctx context.Context) error {
	counter, _ := ctx.Value(checkMultiCounterKey{}).(*checkMultiCounter)
	counter.count++
	if counter.count > counter.maxChecks {
		return fmt.Errorf("number of checks (%d) exceeds max checks limit (%d)", counter.count, counter.maxChecks)
	}

	return nil
}

// runChildCheck executes a single child check and returns its result.
func (l *CheckMulti) runChildCheck(ctx context.Context, snc *Agent, chk multiChildCheck) (*CheckResult, error) {
	tokens := utils.Tokenize(chk.cmdStr)
	tokens, err := utils.TrimQuotesList(tokens)

	if err != nil || len(tokens) == 0 {
		return nil, fmt.Errorf("failed to parse check command: %s", chk.cmdStr)
	}

	cmdName := tokens[0]
	cmdArgs := tokens[1:]

	_, isKnown := snc.getCheck(cmdName, false)

	if chk.isInline && !isKnown {
		return nil, fmt.Errorf("unknown check command: %s (inline checks only support existing check commands)", cmdName)
	}

	if isKnown {
		return snc.RunCheckWithContext(ctx, cmdName, cmdArgs, 0, nil, false), nil
	}

	timeout := l.externalScriptTimeout(snc)
	stdout, stderr, exitCode, err := snc.runExternalCheckString(ctx, chk.cmdStr, timeout)
	out := stdout
	if stderr != "" && !strings.Contains(out, stderr) {
		if out != "" {
			out += "\n"
		}
		out += "[" + stderr + "]"
	}
	res := &CheckResult{
		State:     exitCode,
		Output:    out,
		IsTimeout: errors.Is(err, context.DeadlineExceeded),
	}
	res.ParsePerformanceDataFromOutput()

	return res, nil
}
