package snclient

import (
	"context"
	"fmt"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/utils"
)

func init() {
	AvailableChecks["check_multi"] = CheckEntry{"check_multi", NewCheckMulti}
}

type CheckMulti struct {
	checks []string
	config string
}

func NewCheckMulti() CheckHandler {
	return &CheckMulti{
		checks: make([]string, 0),
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
    check_process process=123
    check_process process=345

    This can be executed with 'check_multi "config=mycheck"'.

	It's also possible to use custom scripts in the config section, for example:
	[/settings/check/multi/myscript]
	/path/to/plugin1
	/path/to/plugin2
	/path/to/plugin3

	This can be executed with 'check_multi "config=myscript"'.
`,
		implemented:   ALL,
		disableFilter: true,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"check":  {value: &l.checks, description: "Check command to execute (can be specified multiple times)"},
			"config": {value: &l.config, description: "Config section name under [/settings/check/multi/< section >] to execute"},
		},
		conditionAlias: map[string]map[string]string{
			"warning_count":  {"warn_count": "warning_count"},
			"critical_count": {"crit_count": "critical_count"},
		},
		attributes: []CheckAttribute{
			{name: "count", description: "Total number of checks executed", unit: UNone},
			{name: "ok_count", description: "Number of checks in OK state", unit: UNone},
			{name: "warning_count", description: "Number of checks in WARNING state", unit: UNone},
			{name: "critical_count", description: "Number of checks in CRITICAL state", unit: UNone},
			{name: "unknown_count", description: "Number of checks in UNKNOWN state", unit: UNone},
			{name: "problem_count", description: "Number of checks in non-OK state", unit: UNone},
			{name: "name", description: "Name/tag of the check", unit: UNone},
			{name: "command", description: "Command executed", unit: UNone},
			{name: "state", description: "Exit code of the check (0=OK, 1=WARNING, 2=CRITICAL, 3=UNKNOWN)", unit: UNone},
			{name: "status", description: "Status text of the check (OK, WARNING, CRITICAL, UNKNOWN)", unit: UNone},
			{name: "output", description: "Output of the check", unit: UNone},
		},
		defaultWarning:  "warning_count > 0",
		defaultCritical: "critical_count > 0 || unknown_count > 0",
		okSyntax:        "%(status) - %(count) plugins checked, %(ok_count) ok",
		topSyntax:       "%(status) - %(count) plugins checked: %(ok_count) ok, %(warning_count) warning, %(critical_count) critical, %(unknown_count) unknown%(problem_list)",
		detailSyntax:    "[%(status)] %(name): %(output)",
		emptySyntax:     "%(status) - no checks executed",
		emptyState:      CheckExitUnknown,
		exampleDefault: `
    check_multi "check=check_process 'process=firefox'" "check=check_memory 'crit=used_pct gt 80%'"
	OK - 2 plugins checked, 2 ok | 'check_process::count'=1;;;0 'check_process::rss'=258686976B;;;0 ...
	[ 1] check_process OK - all 1 processes are ok.
	[ 2] check_memory OK - physical = 12.22 GiB/16.00 GiB (76.4%), swap = 1.95 GiB/3.00 GiB (65.0%)

	You can define 'warning' and 'critical' conditions based on the number of checks in a certain state (see attributes below):

	check_multi "check=check_dummy 0 'OK'" "check=check_dummy 1 'WARNING'" "critical=problem_count gt 0"
	CRITICAL - 2 plugins checked: 1 ok, 1 warning, 0 critical, 0 unknown
	[ 1] check_dummy OK
	[ 2] check_dummy WARNING

	You can also override the 'top-syntax' and use IF ELSE statements to get a certain output based on the results:

	check_multi "check=check_dummy 0 'OK'" "check=check_dummy 2 'CRITICAL'" \
				"top-syntax={{ if ok_count gt 0 }}OK - %(ok_count)/%(count) checks are OK {{ ELSE }}CRITICAL - all checks failed{{ END }}"
	OK - 1/2 checks are OK
	[ 1] check_dummy OK
	[ 2] check_dummy CRITICAL
	`,
	}
}

type multiChildCheck struct {
	tag      string
	cmdStr   string
	isInline bool
}

func (l *CheckMulti) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	enabled, _, _ := snc.config.Section("/modules").GetBool("CheckMulti")
	if !enabled {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: "module CheckMulti is not enabled in /modules section",
		}, nil
	}

	maxChecks, ok, err := snc.config.Section("/settings/check/multi").GetInt("max checks")
	if err != nil || !ok || maxChecks <= 0 {
		maxChecks = 20
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

	if int64(len(childChecks)) > maxChecks {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: fmt.Sprintf("number of checks (%d) exceeds max checks limit (%d)", len(childChecks), maxChecks),
		}, nil
	}

	return l.executeChildChecks(ctx, snc, check, childChecks)
}

// buildChildChecks assembles the list of child checks from config section and inline args.
func (l *CheckMulti) buildChildChecks(snc *Agent) ([]multiChildCheck, *CheckResult) {
	childChecks := []multiChildCheck{}

	if l.config != "" {
		configChecks, res := l.buildConfigChecks(snc)
		if res != nil {
			return nil, res
		}
		childChecks = append(childChecks, configChecks...)
	}

	for _, inlineCmd := range l.checks {
		inlineCmd = strings.TrimSpace(inlineCmd)
		if inlineCmd == "" {
			continue
		}
		childChecks = append(childChecks, multiChildCheck{
			tag:      "",
			cmdStr:   inlineCmd,
			isInline: true,
		})
	}

	return childChecks, nil
}

// buildConfigChecks loads checks from the named config section.
func (l *CheckMulti) buildConfigChecks(snc *Agent) ([]multiChildCheck, *CheckResult) {
	secName := "/settings/check/multi/" + l.config
	sec := snc.config.Section(secName)

	if len(sec.keys) == 0 {
		return nil, &CheckResult{
			State:  CheckExitUnknown,
			Output: fmt.Sprintf("no checks defined in config section %s", secName),
		}
	}

	childChecks := make([]multiChildCheck, 0, len(sec.keys))

	for _, key := range sec.keys {
		rawCmd, tag := l.resolveConfigEntry(snc, key, sec.data[key])
		childChecks = append(childChecks, multiChildCheck{
			tag:      tag,
			cmdStr:   rawCmd,
			isInline: false,
		})
	}

	return childChecks, nil
}

// resolveConfigEntry determines the raw command and tag for a single config section entry.
func (l *CheckMulti) resolveConfigEntry(snc *Agent, key, val string) (rawCmd, tag string) {
	rawCmd = key
	tag = ""

	if val == "" {
		return rawCmd, tag
	}

	if _, isKnown := snc.getCheck(key, false); isKnown {
		return key + " " + val, key
	}

	return val, key
}

// executeChildChecks runs all child checks and aggregates results.
func (l *CheckMulti) executeChildChecks(ctx context.Context, snc *Agent, check *CheckData, childChecks []multiChildCheck) (*CheckResult, error) {
	var count, okCount, warnCount, critCount, unknownCount int64

	detailsList := make([]string, 0, len(childChecks))
	allMetrics := make([]*CheckMetric, 0)

	for idx, chk := range childChecks {
		res, fatal := l.runChildCheck(ctx, snc, check, chk)
		if fatal {
			return res, nil
		}

		count++
		switch res.State {
		case CheckExitOK:
			okCount++
		case CheckExitWarning:
			warnCount++
		case CheckExitCritical:
			critCount++
		default:
			unknownCount++
		}

		tokens := utils.Tokenize(chk.cmdStr)
		cmdName := chk.cmdStr
		if len(tokens) > 0 {
			cmdName = tokens[0]
		}

		tag := chk.tag
		if tag == "" {
			tag = cmdName
		}

		firstLine := strings.TrimSpace(strings.Split(res.Output, "\n")[0])
		detailsList = append(detailsList, fmt.Sprintf("[% 2d] %s %s", idx+1, tag, res.Output))

		entry := map[string]string{
			"idx":     fmt.Sprintf("%d", idx+1),
			"name":    tag,
			"command": chk.cmdStr,
			"state":   fmt.Sprintf("%d", res.State),
			"status":  res.StateString(),
			"output":  firstLine,
			"_state":  fmt.Sprintf("%d", res.State),
			"_skip":   "1",
			"_count":  "1",
		}
		check.listData = append(check.listData, entry)

		for _, m := range res.Metrics {
			metricCopy := *m
			metricCopy.Name = fmt.Sprintf("%s::%s", tag, m.Name)
			allMetrics = append(allMetrics, &metricCopy)
		}
	}

	problemCount := warnCount + critCount + unknownCount
	check.details = map[string]string{
		"count":          fmt.Sprintf("%d", count),
		"ok_count":       fmt.Sprintf("%d", okCount),
		"warning_count":  fmt.Sprintf("%d", warnCount),
		"warn_count":     fmt.Sprintf("%d", warnCount),
		"critical_count": fmt.Sprintf("%d", critCount),
		"crit_count":     fmt.Sprintf("%d", critCount),
		"unknown_count":  fmt.Sprintf("%d", unknownCount),
		"problem_count":  fmt.Sprintf("%d", problemCount),
	}

	check.result.Metrics = allMetrics
	check.result.Details = strings.Join(detailsList, "\n")

	return check.Finalize()
}

// runChildCheck executes a single child check and returns its result.
// The second return value is true when the error is fatal and the caller should stop processing.
func (l *CheckMulti) runChildCheck(ctx context.Context, snc *Agent, check *CheckData, chk multiChildCheck) (*CheckResult, bool) {
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

	stdout, stderr, exitCode, err := snc.runExternalCheckString(ctx, chk.cmdStr, int64(check.timeout))
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
