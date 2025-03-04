package snclient

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
)

func setupConfig(t *testing.T, scriptsDir, scriptsType string) string {
	t.Helper()

	config := fmt.Sprintf(`
[/modules]
CheckExternalScripts = enabled

[/paths]
scripts = %s
shared-path = %%(scripts)

[/settings/external scripts/wrappings]
sh = %%SCRIPT%% %%ARGS%%
exe = %%SCRIPT%% %%ARGS%%

[/settings/external scripts]
timeout = 1111111
[/settings/external scripts/scripts]
check_doesnotexist = /a/path/that/does/not/exist/nonexisting_script "no" "no"
check_cwd_rel_win = cmd /c type pluginoutput
check_cwd_abs_win = cmd /c type ${scripts}\pluginoutput
check_spaceargs_win = cmd /c echo '%%ARG1%%'

check_cwd_rel_unix = cat pluginoutput
check_cwd_abs_unix = cat ${scripts}/pluginoutput
check_spaceargs_unix = echo '%%ARG1%%'

check_dummy = check_dummy.EXTENSION
check_dummy_ok = check_dummy.EXTENSION 0 "i am ok"
check_dummy_critical = check_dummy.EXTENSION 2 "i am critical"
check_dummy_unknown = check_dummy.EXTENSION 3
check_dummy_arg = check_dummy.EXTENSION "$ARG1$" "$ARG2$"
check_dummy_arg2 = check_dummy.EXTENSION "$ARG1$" '$ARG2$'
# for scripts with variable arguments
check_dummy_args = check_dummy.EXTENSION $ARGS$
# for scripts with variable arguments, %%ARGS%% is an alternative to $ARGS$
# but it is fairly undocumented and should not be used imho
check_dummy_args%% = check_dummy.EXTENSION %%ARGS%%
# put variable arguments in quotes
check_dummy_argsq = check_dummy.EXTENSION $ARGS"$
check_dummy_subdir = subdir/check_dummy.EXTENSION 0 "i am ok in my subdir"

# test some issues
check_win_none_ex = cmd /c echo scripts\custom\wrapper\timeout.ps1 $ARG1$; exit($lastexitcode) | powershell.exe -ExecutionPolicy ByPass -command -
check_win_subargs = t\scripts\check_args.ps1 $ARG1$

[/settings/external scripts/wrapped scripts]
check_dummy_wrapped_noparm = check_dummy.EXTENSION
check_dummy_wrapped = check_dummy.EXTENSION $ARG1$ "$ARG2$"
check_dummy_wrapped_args = check_dummy.EXTENSION $ARGS"$
check_dummy_wrapped_ok = check_dummy.EXTENSION 0 "i am ok wrapped"
check_dummy_wrapped_critical = check_dummy.EXTENSION 2 "i am critical wrapped"

[/settings/external scripts/scripts/timeoutscript_win]
timeout = 1
command = ping 127.0.0.1 -n 10

[/settings/external scripts/scripts/timeoutscript_unix]
timeout = 1
command = sleep 10
`, scriptsDir)
	config = strings.ReplaceAll(config, "EXTENSION", scriptsType)

	return config
}

func setupTeardown(t *testing.T, arg string) func() {
	t.Helper()
	// teardown function
	return func() {
		err := os.RemoveAll(arg)
		if err != nil {
			return
		}
	}
}

func runTestCheckExternalDefault(t *testing.T, snc *Agent) {
	t.Helper()

	res := snc.RunCheck("check_dummy", []string{})
	assert.Equalf(t, CheckExitUnknown, res.State, "state matches")
	assert.Equalf(t, "Invalid state argument. Please provide one of: 0, 1, 2, 3", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_ok", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "OK: i am ok", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_ok", []string{"0", "i am ok ignored"})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "OK: i am ok", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_critical", []string{})
	assert.Equalf(t, CheckExitCritical, res.State, "state matches")
	assert.Equalf(t, "CRITICAL: i am critical", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_critical", []string{"2", "i am critical ignored"})
	assert.Equalf(t, CheckExitCritical, res.State, "state matches")
	assert.Equalf(t, "CRITICAL: i am critical", string(res.BuildPluginOutput()), "output matches")

	// providing arguments here does not replace the arguments in the ini
	res = snc.RunCheck("check_dummy_unknown", []string{"3", "i am unknown ignored"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state matches")
	assert.Equalf(t, "UNKNOWN", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_arg", []string{"0", "arg ok"})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "OK: arg ok", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_arg", []string{"0"})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "OK", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_arg2", []string{"0"})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "OK", string(res.BuildPluginOutput()), "output matches")
}

func runTestCheckExternalArgs(t *testing.T, snc *Agent) {
	t.Helper()

	res := snc.RunCheck("check_dummy_args", []string{"0", "arg ok"})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	// $ARGS$ makes "0", "arg ok" a flat list: 0 arg ok
	assert.Equalf(t, "OK: arg", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_args%", []string{"0", "arg ok"})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	// %ARGS% makes "0", "arg ok" a flat list: 0 arg ok
	assert.Equalf(t, "OK: arg", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_argsq", []string{"0", "arg ok"})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	// $ARG"S$ makes "0", "arg ok" a quoted list: "0" "arg ok"
	assert.Equalf(t, "OK: arg ok", string(res.BuildPluginOutput()), "output matches")
}

func runTestCheckExternalWrapped(t *testing.T, snc *Agent) {
	t.Helper()

	res := snc.RunCheck("check_dummy_wrapped_noparm", []string{"0", "i am wrapped"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state matches")
	assert.Equalf(t, "Invalid state argument. Please provide one of: 0, 1, 2, 3", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_wrapped", []string{"0", "i am wrapped"})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "OK: i am wrapped", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_wrapped_args", []string{"0", "i am wrapped"})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "OK: i am wrapped", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_wrapped_ok", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "OK: i am ok wrapped", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_wrapped_critical", []string{"0", "critical ignored"})
	assert.Equalf(t, CheckExitCritical, res.State, "state matches")
	assert.Equalf(t, "CRITICAL: i am critical wrapped", string(res.BuildPluginOutput()), "output matches")
}

func TestMain(m *testing.M) {
	cmd := exec.Command("go", "build", "-o", "check_dummy.exe", "check_dummy.go")
	cmd.Dir = "t/scripts"
	_, _ = cmd.CombinedOutput()
	if runtime.GOOS != "windows" {
		_ = os.Mkdir("t/scripts/subdir", os.ModePerm)
		cmd = exec.Command("go", "build", "-o", "subdir/check_dummy.exe", "check_dummy.go")
		cmd.Dir = "t/scripts"
		_, _ = cmd.CombinedOutput()
	}

	// run the tests
	exitCode := m.Run()

	// run teardown code
	_ = os.Remove("t/scripts/check_dummy.exe")
	_ = os.RemoveAll("t/scripts/subdir")

	// exit with the same exit code as the tests
	os.Exit(exitCode)
}

func TestCheckExternalCwd(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(t, scriptsDir, "sh")
	snc := StartTestAgent(t, config)

	suffix := "unix"
	if runtime.GOOS == "windows" {
		suffix = "win"
	}

	res := snc.RunCheck("check_cwd_rel_"+suffix, []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "i am in the scripts folder", string(res.BuildPluginOutput()), "output matches")
	res = snc.RunCheck("check_cwd_abs_"+suffix, []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "i am in the scripts folder", string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}

func TestCheckExternalTimeout(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(t, scriptsDir, "sh")
	snc := StartTestAgent(t, config)

	suffix := "unix"
	if runtime.GOOS == "windows" {
		suffix = "win"
	}

	res := snc.RunCheck("timeoutscript_"+suffix, []string{})
	assert.Equalf(t, CheckExitUnknown, res.State, "state matches")
	assert.Equalf(t, "UNKNOWN - script run into timeout after 1s\n", string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}

func TestCheckExternalArgSpaces(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")
	holesDir := filepath.Join(testDir, "t", "scri pts")

	suffix := "unix"
	if runtime.GOOS == "windows" {
		suffix = "win"
	}

	teardown := setupTeardown(t, holesDir)
	defer teardown()
	_ = copy.Copy(scriptsDir, holesDir)

	config := setupConfig(t, holesDir, "sh")
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_spaceargs_"+suffix, []string{""})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	// windows returns double quotes somehow
	// PS C:\> cmd /c echo ' '
	// " "
	if runtime.GOOS == "windows" {
		assert.Equalf(t, `""`, string(res.BuildPluginOutput()), "output matches")
	} else {
		assert.Equalf(t, "", string(res.BuildPluginOutput()), "output matches")
	}

	StopTestAgent(t, snc)
}

func TestCheckExternalExe(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(t, scriptsDir, "exe")
	snc := StartTestAgent(t, config)

	runTestCheckExternalDefault(t, snc)
	runTestCheckExternalArgs(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalExePathWithSpaces(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")
	holesDir := filepath.Join(testDir, "t", "scri pts")

	teardown := setupTeardown(t, holesDir)
	defer teardown()
	_ = copy.Copy(scriptsDir, holesDir)

	config := setupConfig(t, holesDir, "exe")
	snc := StartTestAgent(t, config)

	runTestCheckExternalDefault(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalNonExist(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(t, scriptsDir, "sh")
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_doesnotexist", []string{})
	assert.Equalf(t, CheckExitUnknown, res.State, "state matches")
	assert.Regexp(t, "UNKNOWN - Return code of 127.*actually exists", string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}
