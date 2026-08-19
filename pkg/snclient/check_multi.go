package snclient

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/utils"
)

func init() {
	AvailableChecks["check_multi"] = CheckEntry{"check_multi", NewCheckMulti}
}

type (
	checkMultiConfigKey struct{}
	checkMultiDepthKey  struct{}
)

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
		okSyntax:        "%(status) - %(count) plugins checked, %(ok_count) ok",
		topSyntax:       "%(status) - %(count) plugins checked: %(ok_count) ok, %(warning_count) warning, %(critical_count) critical, %(unknown_count) unknown - %(problem_list)",
		detailSyntax:    "%(name): %(output)",
		emptySyntax:     "%(status) - no checks executed",
		emptyState:      CheckExitUnknown,
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

func (l *CheckMulti) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	enabled, _, _ := snc.config.Section("/modules").GetBool("CheckMulti")
	if !enabled {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: "module CheckMulti is not enabled in /modules section",
		}, nil
	}

	depth, _ := ctx.Value(checkMultiDepthKey{}).(int)
	if depth > 5 {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: "recursion limit exceeded for check_multi",
		}, nil
	}
	ctx = context.WithValue(ctx, checkMultiDepthKey{}, depth+1)

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
	sec := snc.config.Section(secName)

	if len(sec.keys) == 0 {
		return nil, &CheckResult{
			State:  CheckExitUnknown,
			Output: fmt.Sprintf("no checks defined in config section %s", secName),
		}
	}

	childChecks := make([]multiChildCheck, 0, len(sec.keys))
	seenTags := make(map[string]bool)

	for _, key := range sec.keys {
		rawVal := sec.data[key]
		if !strings.HasPrefix(key, "command[") || !strings.HasSuffix(key, "]") {
			return nil, &CheckResult{
				State:  CheckExitUnknown,
				Output: fmt.Sprintf("invalid check_multi config entry: %s (must be in format command[tag]=<command>)", key),
			}
		}
		tag := strings.TrimSuffix(strings.TrimPrefix(key, "command["), "]")
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
		if seenTags[tag] {
			return nil, &CheckResult{
				State:  CheckExitUnknown,
				Output: fmt.Sprintf("duplicate command tag: %s", tag),
			}
		}
		seenTags[tag] = true
		childChecks = append(childChecks, multiChildCheck{
			tag:      tag,
			cmdStr:   rawVal,
			isInline: false,
		})
	}

	return childChecks, nil
}

// executeChildChecks runs all child checks and aggregates results.
func (l *CheckMulti) executeChildChecks(ctx context.Context, snc *Agent, check *CheckData, childChecks []multiChildCheck) (*CheckResult, error) {
	var count, okCount, warnCount, critCount, unknownCount int64

	detailsList := make([]string, 0, len(childChecks))
	allMetrics := make([]*CheckMetric, 0)

	hasEntryThresholds := check.HasThreshold("name") || check.HasThreshold("tag") || check.HasThreshold("command") ||
		check.HasThreshold("output") || check.HasThreshold("shortoutput") || check.HasThreshold("status") || check.HasThreshold("state")

	for _, chk := range childChecks {
		res, fatal := l.runChildCheck(ctx, snc, check, chk)
		if fatal {
			return res, nil
		}

		tag := chk.tag

		firstLine := strings.TrimSpace(strings.Split(res.Output, "\n")[0])
		detailsList = append(detailsList, fmt.Sprintf("[%s] %s", tag, res.Output))

		entryState := fmt.Sprintf("%d", res.State)
		entry := map[string]string{
			"name":        tag,
			"tag":         tag,
			"command":     chk.cmdStr,
			"state":       entryState,
			"status":      res.StateString(),
			"shortoutput": firstLine,
			"output":      res.Output,
			"_state":      entryState,
			"_skip":       "1",
			"_count":      "1",
		}

		if hasEntryThresholds {
			check.Check(entry, check.warnThreshold, check.critThreshold, check.unknownThreshold, check.okThreshold)
		}

		count++
		switch entry["_state"] {
		case "0":
			okCount++
		case "1":
			warnCount++
		case "2":
			critCount++
		default:
			unknownCount++
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

	stdout, stderr, exitCode, _ := snc.runExternalCheckString(ctx, chk.cmdStr, int64(check.timeout))
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
