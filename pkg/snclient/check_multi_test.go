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

	// 1. Basic inline checks - all OK
	res := snc.RunCheck("check_multi", []string{
		"check=check_dummy 0 'dummy ok 1'",
		"check=check_dummy 0 'dummy ok 2'",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, res.Output, "2 plugins checked, 2 ok")
	assert.Contains(t, res.Details, "dummy ok 1")
	assert.Contains(t, res.Details, "dummy ok 2")

	// 2. Inline checks with warning and critical (default thresholds)
	res = snc.RunCheck("check_multi", []string{
		"check=check_dummy 0 'dummy ok'",
		"check=check_dummy 1 'dummy warn'",
	})
	assert.Equalf(t, CheckExitWarning, res.State, "state WARNING")
	assert.Contains(t, res.Output, "2 plugins checked: 1 ok, 1 warning, 0 critical, 0 unknown")

	res = snc.RunCheck("check_multi", []string{
		"check=check_dummy 0 'dummy ok'",
		"check=check_dummy 2 'dummy crit'",
	})
	assert.Equalf(t, CheckExitCritical, res.State, "state CRITICAL")
	assert.Contains(t, res.Output, "2 plugins checked: 1 ok, 0 warning, 1 critical, 0 unknown")

	// 3. Custom conditions: warn=none crit=ok_count ne 2
	res = snc.RunCheck("check_multi", []string{
		"check=check_dummy 0 'dummy 1'",
		"check=check_dummy 0 'dummy 2'",
		"warn=none",
		"crit=ok_count ne 2",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK when ok_count == 2")

	res = snc.RunCheck("check_multi", []string{
		"check=check_dummy 0 'dummy 1'",
		"check=check_dummy 1 'dummy 2'",
		"warn=none",
		"crit=ok_count ne 2",
	})
	assert.Equalf(t, CheckExitCritical, res.State, "state CRITICAL when ok_count != 2")

	// 4. Custom conditions: warn=problem_count gt 0
	res = snc.RunCheck("check_multi", []string{
		"check=check_dummy 0 'dummy 1'",
		"check=check_dummy 1 'dummy 2'",
		"warn=problem_count gt 0",
		"crit=none",
	})
	assert.Equalf(t, CheckExitWarning, res.State, "state WARNING when problem_count > 0")

	// 5. Unknown/inline checks restriction (cannot run arbitrary external commands inline)
	res = snc.RunCheck("check_multi", []string{
		"check=/bin/nonexistent_or_external_script -H 123",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN for unregistered inline command")
	assert.Contains(t, res.Output, "unknown check command")

	// 6. Inline check with check_process or check_cpu
	res = snc.RunCheck("check_multi", []string{
		"check=check_cpu warn=load=101 crit=load=102",
		"warn=none",
		"crit=ok_count ne 1",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK for check_cpu inline")
	assert.Contains(t, res.Details, "check_cpu")

	// 7. Filter argument is disabled/rejected
	res = snc.RunCheck("check_multi", []string{
		"check=check_dummy 0 'ok'",
		"filter=state=1",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN when filter argument is used")
	assert.Contains(t, res.Output, "filter is disabled for this check")

	// 8. Unknown threshold (default and custom)
	res = snc.RunCheck("check_multi", []string{
		"check=check_dummy 0 'ok'",
		"check=check_dummy 3 'unknown check'",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN when child check is unknown by default")

	res = snc.RunCheck("check_multi", []string{
		"check=check_dummy 0 'ok'",
		"check=check_dummy 3 'unknown check'",
		"unknown=unknown_count gt 0",
		"warning=warning_count gt 0",
		"critical=critical_count gt 0",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN when custom unknown condition matches")

	res = snc.RunCheck("check_multi", []string{
		"check=check_dummy 1 'warn check'",
		"check=check_dummy 3 'unknown check'",
		"unknown=unknown_count gt 0",
		"warning=warning_count gt 0",
		"critical=critical_count gt 0",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN takes precedence over WARNING")

	res = snc.RunCheck("check_multi", []string{
		"check=check_dummy 2 'crit check'",
		"check=check_dummy 3 'unknown check'",
		"unknown=unknown_count gt 0",
		"warning=warning_count gt 0",
		"critical=critical_count gt 0",
	})
	assert.Equalf(t, CheckExitCritical, res.State, "state CRITICAL takes precedence over UNKNOWN")
}

func TestCheckMultiLimits(t *testing.T) {
	config := `
[/modules]
CheckMulti = enabled

[/settings/check/multi]
max checks = 2
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	// Under limit: 2 checks
	res := snc.RunCheck("check_multi", []string{
		"check=check_dummy 0 'ok 1'",
		"check=check_dummy 0 'ok 2'",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK for 2 checks")

	// Exceeds limit: 3 checks
	res = snc.RunCheck("check_multi", []string{
		"check=check_dummy 0 'ok 1'",
		"check=check_dummy 0 'ok 2'",
		"check=check_dummy 0 'ok 3'",
	})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN when exceeding max checks")
	assert.Contains(t, res.Output, "exceeds max checks limit")
}

func TestCheckMultiDisabled(t *testing.T) {
	config := `
[/modules]
CheckMulti = disabled
`
	snc := StartTestAgent(t, config)
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_multi", []string{
		"check=check_dummy 0 'ok 1'",
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
check_dummy 0 ok1
check_dummy 0 ok2

[/settings/check/multi/custom]
%s -H 123
%s -W 123

[/settings/check/multi/named]
first = check_dummy 0 ok_first
second = %s -H 456
`, script1, script2, script1)

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

	// Test config=named (named check tags in config)
	res = snc.RunCheck("check_multi", []string{
		"config=named",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK for named config")
	assert.Contains(t, res.Details, "first")
	assert.Contains(t, res.Details, "second")

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
