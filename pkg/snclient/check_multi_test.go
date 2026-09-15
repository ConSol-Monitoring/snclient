package snclient

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckMultiInline(t *testing.T) {
	config := `
[/modules]
CheckMulti = enabled
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	// 1. Basic inline checks with mandatory tags - all OK
	res := snc.RunCheck("check_multi", []string{
		"command[d1]=check_dummy 0 'dummy ok 1'",
		"command[d2]=check_dummy 0 'dummy ok 2'",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, res.Output, "2 plugins checked, 2 ok")
	assert.Contains(t, res.Details, "[d1] dummy ok 1")
	assert.Contains(t, res.Details, "[d2] dummy ok 2")

	// 2. Inline checks with warning and critical (default thresholds)
	res = snc.RunCheck("check_multi", []string{
		"command[d1]=check_dummy 0 'dummy ok'",
		"command[d2]=check_dummy 1 'dummy warn'",
	})
	assert.Equalf(t, CheckExitWarning, res.State, "state WARNING")
	assert.Contains(t, res.Output, "2 plugins checked: 1 ok, 1 warning, 0 critical, 0 unknown")

	res = snc.RunCheck("check_multi", []string{
		"command[test1]=check_dummy 1 WARN",
		"warn=none",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK when warning threshold is none")
	assert.Contains(t, res.Output, "OK - 1 plugins checked: 0 ok, 1 warning, 0 critical, 0 unknown - test1: WARN")

	res = snc.RunCheck("check_multi", []string{
		"command[d1]=check_dummy 0 'dummy ok'",
		"command[d2]=check_dummy 2 'dummy crit'",
	})
	assert.Equalf(t, CheckExitCritical, res.State, "state CRITICAL")
	assert.Contains(t, res.Output, "2 plugins checked: 1 ok, 0 warning, 1 critical, 0 unknown")

	// 3. Custom conditions: warn=none crit=ok_count ne 2
	res = snc.RunCheck("check_multi", []string{
		"command[d1]=check_dummy 0 'dummy 1'",
		"command[d2]=check_dummy 0 'dummy 2'",
		"warn=none",
		"crit=ok_count ne 2",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK when ok_count == 2")

	res = snc.RunCheck("check_multi", []string{
		"command[d1]=check_dummy 0 'dummy 1'",
		"command[d2]=check_dummy 1 'dummy 2'",
		"warn=none",
		"crit=ok_count ne 2",
	})
	assert.Equalf(t, CheckExitCritical, res.State, "state CRITICAL when ok_count != 2")

	// 4. Custom condition on entry attribute: critical=name eq 'alias2' and state=2
	res = snc.RunCheck("check_multi", []string{
		"command[alias1]=check_dummy 2 'crit 1'",
		"command[alias2]=check_dummy 0 'ok 2'",
		"warn=none",
		"crit=name eq 'alias2' and state=2",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK when alias2 is not in state 2")

	res = snc.RunCheck("check_multi", []string{
		"command[alias1]=check_dummy 0 'ok 1'",
		"command[alias2]=check_dummy 2 'crit 2'",
		"warn=none",
		"crit=name eq 'alias2' and state=2",
	})
	assert.Equalf(t, CheckExitCritical, res.State, "state CRITICAL when alias2 is in state 2")

	// 5. Mandatory tag validation: missing tag & duplicate tag
	res = snc.RunCheck("check_multi", []string{
		"command=check_dummy 0 'ok'",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN when command has no tag")
	assert.Contains(t, res.Output, "command argument requires a unique tag")

	res = snc.RunCheck("check_multi", []string{
		"command[dup]=check_dummy 0 'ok 1'",
		"command[dup]=check_dummy 0 'ok 2'",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN when command tag is duplicated")
	assert.Contains(t, res.Output, "duplicate command tag: dup")

	// 6. Unknown/inline checks restriction (cannot run arbitrary external commands inline)
	res = snc.RunCheck("check_multi", []string{
		"command[ext]=/bin/nonexistent_or_external_script -H 123",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN for unregistered inline command")
	assert.Contains(t, res.Output, "unknown check command")

	// 7. Filter argument is disabled/rejected
	res = snc.RunCheck("check_multi", []string{
		"command[d1]=check_dummy 0 'ok'",
		"filter=state=1",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN when filter argument is used")
	assert.Contains(t, res.Output, "filter is disabled for this check")

	// 8. Severity hierarchy: UNKNOWN > CRITICAL > WARNING > OK
	res = snc.RunCheck("check_multi", []string{
		"command[d1]=check_dummy 0 'ok'",
		"command[d2]=check_dummy 3 'unknown check'",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN when child check is unknown by default")

	res = snc.RunCheck("check_multi", []string{
		"command[d1]=check_dummy 1 'warn check'",
		"command[d2]=check_dummy 3 'unknown check'",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN takes precedence over WARNING")

	res = snc.RunCheck("check_multi", []string{
		"command[d1]=check_dummy 2 'crit check'",
		"command[d2]=check_dummy 3 'unknown check'",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN takes precedence over CRITICAL")
}

func TestCheckMultiDefaultEnabled(t *testing.T) {
	snc := StartTestAgent(t, "")
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_multi", []string{
		"command[d1]=check_dummy 0 'default enabled'",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK when CheckMulti is enabled by default")
}

func TestCheckMultiGlobalTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the Unix sleep command")
	}

	config := `
[/modules]
CheckMulti = enabled

[/settings/default]
timeout = 1

[/settings/external scripts]
timeout = 5

[/settings/check/multi/timeout]
command[slow] = sleep 3; echo OK
command[never] = check_dummy 0 'must not run'
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_multi", []string{
		"config=timeout",
	})

	assert.Equal(t, CheckExitUnknown, res.State)
	assert.Equal(t, "UNKNOWN - check_multi timed out after 1s", res.Output)
	assert.Contains(t, res.Details, "[slow] timed out after 1s (reached check_multi timeout of 1s)")
	assert.NotContains(t, res.BuildOutputString(), "must not run")
}

func TestCheckMultiGlobalTimeoutPreservesLiteralChildOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the Unix sleep command")
	}

	config := `
[/modules]
CheckMulti = enabled

[/settings/default]
timeout = 1

[/settings/check/multi/timeout]
command[literal] = check_dummy 0 '%(count) {{ IF condition }}literal{{ END }}'
command[slow] = sleep 3
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_multi", []string{"config=timeout"})

	assert.Equal(t, CheckExitUnknown, res.State)
	assert.Contains(t, res.Details, "[literal] %(count) {{ IF condition }}literal{{ END }} (took ")
}

func TestCheckMultiCustomScriptTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the Unix sleep command")
	}

	config := `
[/modules]
CheckMulti = enabled

[/settings/default]
timeout = 5

[/settings/external scripts]
timeout = 1

[/settings/check/multi/timeout]
command[slow] = sleep 5
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_multi", []string{
		"config=timeout",
	})

	assert.Equal(t, CheckExitUnknown, res.State)
	assert.Contains(t, res.Output, "UNKNOWN - 1 plugins checked: 0 ok, 0 warning, 0 critical, 1 unknown - slow: UNKNOWN - script run into timeout after 1s")
	assert.Contains(t, res.Details, "[slow] UNKNOWN - script run into timeout after 1s (reached external scripts timeout of 1s)")
	assert.NotContains(t, res.Details, "(took ")
}

func TestCheckMultiNamedExternalScriptUsesParentTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the Unix sleep command")
	}

	config := `
[/modules]
CheckMulti = enabled
CheckExternalScripts = enabled

[/settings/default]
timeout = 3

[/settings/external scripts]
timeout = 2

[/settings/external scripts/scripts]
check_ext = sleep 10

[/settings/check/multi/timeout]
command[check_ext1] = check_ext
command[check_ext2] = check_ext
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_multi", []string{"config=timeout"})

	assert.Equal(t, CheckExitUnknown, res.State)
	assert.Equal(t, "UNKNOWN - check_multi timed out after 3s", res.Output)
	assert.Contains(t, res.Details, "[check_ext1] UNKNOWN - script run into timeout after 2s (took ")
	assert.Contains(t, res.Details, "[check_ext2] timed out after 1s (reached check_multi timeout of 3s)")
}

func TestCheckMultiLaterChildUsesRemainingTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the Unix sleep command")
	}

	config := `
[/modules]
CheckMulti = enabled

[/settings/default]
timeout = 3

[/settings/external scripts]
timeout = 10

[/settings/check/multi/timeout]
command[check1] = /bin/sh -c 'sleep 1; echo OK'
command[check2] = sleep 4
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_multi", []string{
		"config=timeout",
	})

	assert.Equal(t, CheckExitUnknown, res.State)
	assert.Equal(t, "UNKNOWN - check_multi timed out after 3s", res.Output)
	assert.Regexp(t, `\[check1\] OK \(took 1(?:\.\d+)?s\)`, res.Details)
	assert.Contains(t, res.Details, "[check2] timed out after 2s (reached check_multi timeout of 3s)")
}

func TestCheckMultiScenarioGlobalTimeoutPartialExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the Unix sleep command")
	}

	config := `
[/modules]
CheckMulti = enabled

[/settings/default]
timeout = 3

[/settings/external scripts]
timeout = 10

[/settings/check/multi/scenario1]
command[check1] = /bin/sh -c 'sleep 1; echo OK'
command[check2] = sleep 4
command[check3] = check_dummy 0 'check 3'
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_multi", []string{
		"config=scenario1",
	})

	assert.Equal(t, CheckExitUnknown, res.State)
	assert.Equal(t, "UNKNOWN - check_multi timed out after 3s", res.Output)
	assert.Regexp(t, `\[check1\] OK \(took 1(?:\.\d+)?s\)`, res.Details)
	assert.Contains(t, res.Details, "[check2] timed out after 2s (reached check_multi timeout of 3s)")
	assert.NotContains(t, res.Details, "[check3]")
}

func TestCheckMultiScenarioExternalTimeoutSummaryAndDetails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the Unix sleep command")
	}

	config := `
[/modules]
CheckMulti = enabled

[/settings/default]
timeout = 5

[/settings/external scripts]
timeout = 2

[/settings/check/multi/scenario2]
command[check_drivesize] = check_dummy 0 'OK - All 1 drive(s) are ok'
command[check_ext1] = sleep 4
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_multi", []string{
		"config=scenario2",
	})

	assert.Equal(t, CheckExitUnknown, res.State)
	assert.Equal(t, "UNKNOWN - 2 plugins checked: 1 ok, 0 warning, 0 critical, 1 unknown - unknown(check_ext1: UNKNOWN - script run into timeout after 2s)", res.Output)
	assert.Contains(t, res.Details, "[check_drivesize] OK - All 1 drive(s) are ok")
	assert.Contains(t, res.Details, "[check_ext1] UNKNOWN - script run into timeout after 2s (reached external scripts timeout of 2s)")
	assert.NotContains(t, res.Details, "(took ")
}

func TestCheckMultiPreservesLiteralChildOutput(t *testing.T) {
	snc := StartTestAgent(t, "")
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_multi", []string{
		"command[child]=check_dummy 0 '%(count) {{ IF condition }}literal{{ END }}'",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK for literal child output")
	assert.Contains(t, res.Details, "%(count) {{ IF condition }}literal{{ END }}")
}

func TestTaggedNonCommandArgumentRejected(t *testing.T) {
	snc := StartTestAgent(t, "")
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_files", []string{"path[x]=/tmp"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN for tagged non-command argument")
	assert.Contains(t, res.Output, "does not support tags")
}

func TestCheckMultiPriorityThreshold(t *testing.T) {
	config := `
[/modules]
CheckMulti = enabled
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	args := []string{
		"command[prio]=check_dummy 2 'priority critical'",
		"command[dummy2]=check_dummy 0 'dummy 2'",
		"command[dummy3]=check_dummy 0 'dummy 3'",
		"warn=none",
		"unknown=none",
		"crit=name eq 'prio' and state ne '0'",
	}
	res := snc.RunCheck("check_multi", args)
	assert.Equalf(t, CheckExitCritical, res.State, "state CRITICAL when prio is not OK")
	assert.Contains(t, res.Details, "[prio] priority critical")

	res = snc.RunCheck("check_multi", []string{
		"command[prio]=check_dummy 0 'priority ok'",
		"command[dummy2]=check_dummy 3 'dummy 2 unknown'",
		"command[dummy3]=check_dummy 2 'dummy 3 critical'",
		"warn=none",
		"unknown=none",
		"crit=name eq 'prio' and state ne '0'",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK when only non-prio checks are problems")
	assert.Contains(t, res.Details, "[dummy2] dummy 2 unknown")
	assert.Contains(t, res.Details, "[dummy3] dummy 3 critical")
}

func TestCheckMultiLimits(t *testing.T) {
	config := `
[/modules]
CheckMulti = enabled

[/settings/check/multi]
max checks = 4

[/settings/check/multi/nested]
command[d1] = check_dummy 0 'nested 1'
command[d2] = check_dummy 0 'nested 2'
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	// Under limit: 2 checks
	res := snc.RunCheck("check_multi", []string{
		"command[d1]=check_dummy 0 'ok 1'",
		"command[d2]=check_dummy 0 'ok 2'",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK for 2 checks")

	// Exceeds limit: 5 checks
	res = snc.RunCheck("check_multi", []string{
		"command[d1]=check_dummy 0 'ok 1'",
		"command[d2]=check_dummy 0 'ok 2'",
		"command[d3]=check_dummy 0 'ok 3'",
		"command[d4]=check_dummy 0 'ok 4'",
		"command[d5]=check_dummy 0 'ok 5'",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN when exceeding max checks")
	assert.Contains(t, res.Output, "exceeds max checks limit")

	// Nested checks share the same cumulative execution count.
	res = snc.RunCheck("check_multi", []string{
		"command[d1]=check_dummy 0 'outer 1'",
		"command[d2]=check_dummy 0 'outer 2'",
		"command[nested]=check_multi config=nested",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN when nested checks exceed max checks")
	assert.Contains(t, res.Details, "number of checks (5) exceeds max checks limit (4)")
}

func TestCheckMultiDisabled(t *testing.T) {
	config := `
[/modules]
CheckMulti = disabled
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_multi", []string{
		"command[d1]=check_dummy 0 'ok 1'",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN when module is disabled")
	assert.Contains(t, res.Output, "module CheckMulti is not enabled")
}

func TestCheckMultiConfigSection(t *testing.T) {
	// Create temporary scripts to test external scripts in config
	tmpDir := t.TempDir()
	var scriptExt string
	var script1Content, script2Content string

	if runtime.GOOS == "windows" {
		scriptExt = ".ps1"
		script1Content = `Write-Output "SCRIPT 1 OK | perf1=10;20;30"
exit 0
`
		script2Content = `Write-Output "SCRIPT 2 WARNING | perf2=50;40;60"
exit 1
`
	} else {
		scriptExt = ".sh"
		script1Content = `#!/bin/sh
echo "SCRIPT 1 OK | perf1=10;20;30"
exit 0
`
		script2Content = `#!/bin/sh
echo "SCRIPT 2 WARNING | perf2=50;40;60"
exit 1
`
	}

	script1 := filepath.Join(tmpDir, "test1"+scriptExt)
	script2 := filepath.Join(tmpDir, "test2"+scriptExt)

	err := os.WriteFile(script1, []byte(script1Content), 0o600)
	require.NoError(t, err)

	err = os.WriteFile(script2, []byte(script2Content), 0o600)
	require.NoError(t, err)

	if runtime.GOOS != "windows" {
		require.NoError(t, os.Chmod(script1, 0o700))
		require.NoError(t, os.Chmod(script2, 0o700))
	}

	config := fmt.Sprintf(`
[/modules]
CheckMulti = enabled

[/settings/check/multi/mycheck]
command[c1] = check_dummy 0 ok1
command[c2] = check_dummy 0 ok2

[/settings/check/multi/custom]
command[s1] = %s -H 123
command[s2] = %s -W 123

[/settings/check/multi/loop]
command[sub] = check_multi config=loop

[/settings/check/multi/loopA]
command[b] = check_multi config=loopB

[/settings/check/multi/loopB]
command[a] = check_multi config=loopA

[/settings/check/multi/inner]
command[leaf] = check_dummy 0 'nested detail'

[/settings/check/multi/duplicate]
command[foo] = check_dummy 0 first
command[ foo ] = check_dummy 0 second
`, script1, script2)

	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	// Test config=mycheck (builtin checks in config)
	res := snc.RunCheck("check_multi", []string{
		"config=mycheck",
		"warn=none",
		"crit=ok_count ne 2",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK for mycheck config")
	assert.Contains(t, res.Output, "2 plugins checked, 2 ok")

	// Test config=custom (external scripts in config)
	res = snc.RunCheck("check_multi", []string{
		"config=custom",
		"warn=problem_count gt 0",
		"crit=none",
	})
	assert.Equalf(t, CheckExitWarning, res.State, "state WARNING for custom config")
	assert.Contains(t, res.Output, "2 plugins checked: 1 ok, 1 warning, 0 critical, 0 unknown")
	assert.Contains(t, res.Details, "SCRIPT 1 OK")
	assert.Contains(t, res.Details, "SCRIPT 2 WARNING")

	// Nested detail output is included in the parent output attribute.
	res = snc.RunCheck("check_multi", []string{
		"command[nested]=check_multi config=inner",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK for nested detail output")
	assert.Contains(t, res.BuildOutputString(), "nested detail")
	assert.Contains(t, res.Details, "nested detail")

	res = snc.RunCheck("check_multi", []string{
		"config=duplicate",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN for whitespace-duplicate config tags")
	assert.Contains(t, res.Output, "duplicate command tag: foo")

	// Test direct loop detection: check_multi config=loop
	res = snc.RunCheck("check_multi", []string{
		"config=loop",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN for loop config")
	assert.Contains(t, res.Output, "loop detected")

	// Test indirect loop detection: loopA -> loopB -> loopA
	res = snc.RunCheck("check_multi", []string{
		"config=loopA",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN for indirect loop")
	assert.Contains(t, res.Output, "loop detected")

	// Test non-existing config
	res = snc.RunCheck("check_multi", []string{
		"config=doesnotexist",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN for missing config section")
	assert.Contains(t, res.Output, "no checks defined in config section")
}

func TestCheckMultiIndex(t *testing.T) {
	config := `
[/modules]
CheckMulti = enabled
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_index", []string{"filter=name = 'check_multi'"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK for check_index")
	assert.Contains(t, res.Output, "check_multi")
}
