//go:build !windows

package snclient

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
)

func setupConfig(scriptsDir, scriptsType string) string {
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
check_cwd_rel = cat pluginoutput
check_cwd_abs = cat ${scripts}/pluginoutput
check_spaceargs = echo '%%ARG1%%'

check_dummy = check_dummy.EXTENSION
check_dummy_ok = check_dummy.EXTENSION 0 "i am ok"
check_dummy_critical = check_dummy.EXTENSION 2 "i am critical"
check_dummy_unknown = check_dummy.EXTENSION 3
check_dummy_arg = check_dummy.EXTENSION "$ARG1$" "$ARG2$"
# for scripts with variable arguments
check_dummy_args = check_dummy.EXTENSION $ARGS$
# for scripts with variable arguments, %%ARGS%% is an alternative to $ARGS$
# but it is fairly undocumented and should not be used imho
check_dummy_args%% = check_dummy.EXTENSION %%ARGS%%
# put variable arguments in quotes
check_dummy_argsq = check_dummy.EXTENSION $ARGS"$
check_dummy_subdir = subdir/check_dummy.EXTENSION 0 "i am ok in my subdir"

[/settings/external scripts/wrapped scripts]
check_dummy_wrapped_noparm = check_dummy.EXTENSION
check_dummy_wrapped = check_dummy.EXTENSION $ARG1$ "$ARG2$"
check_dummy_wrapped_ok = check_dummy.EXTENSION 0 "i am ok wrapped"
check_dummy_wrapped_critical = check_dummy.EXTENSION 2 "i am critical wrapped"

[/settings/external scripts/scripts/timeoutscript]
timeout = 1
command = sleep 10
`, scriptsDir)
	config = strings.ReplaceAll(config, "EXTENSION", scriptsType)

	return config
}

func TestMain(m *testing.M) {
	cmd := exec.Command("go", "build", "-o", "check_dummy.exe", "check_dummy.go")
	cmd.Dir = "t/scripts"
	_, _ = cmd.CombinedOutput()
	_ = os.Mkdir("t/scripts/subdir", os.ModePerm)
	cmd = exec.Command("go", "build", "-o", "subdir/check_dummy.exe", "check_dummy.go")
	cmd.Dir = "t/scripts"
	_, _ = cmd.CombinedOutput()

	// run the tests
	exitCode := m.Run()

	// run teardown code
	_ = os.Remove("t/scripts/check_dummy.exe")
	_ = os.RemoveAll("t/scripts/subdir")

	// exit with the same exit code as the tests
	os.Exit(exitCode)
}

func TestCheckExternalUnixCwd(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(scriptsDir, "sh")
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_cwd_rel", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "i am in the scripts folder", string(res.BuildPluginOutput()), "output matches")
	res = snc.RunCheck("check_cwd_abs", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "i am in the scripts folder", string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}

func TestCheckExternalUnixTimeout(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(scriptsDir, "sh")
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("timeoutscript", []string{})
	assert.Equalf(t, CheckExitUnknown, res.State, "state matches")
	assert.Equalf(t, "UNKNOWN: script run into timeout after 1s\n", string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}

func TestCheckExternalUnixNonExist(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(scriptsDir, "sh")
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_doesnotexist", []string{})
	assert.Equalf(t, CheckExitCritical, res.State, "state matches")
	assert.Regexp(t, "CRITICAL - Return code of 127.*actually exists", string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}

func TestCheckExternalUnixExeInSubdir(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(scriptsDir, "exe")
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_dummy_subdir", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Regexp(t, "OK: i am ok in my subdir", string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}

func TestCheckExternalUnixExe(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(scriptsDir, "exe")
	snc := StartTestAgent(t, config)

	runTestCheckExternalDefault(t, snc)
	runTestCheckExternalArgs(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalUnixExePathWithSpaces(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")
	holesDir := filepath.Join(testDir, "t", "scri pts")

	teardown := setupTeardown(t, holesDir)
	defer teardown()
	_ = copy.Copy(scriptsDir, holesDir)

	config := setupConfig(holesDir, "exe")
	snc := StartTestAgent(t, config)

	runTestCheckExternalDefault(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalUnixShell(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(scriptsDir, "sh")
	snc := StartTestAgent(t, config)

	runTestCheckExternalDefault(t, snc)
	runTestCheckExternalArgs(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalWrappedUnixShell(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(scriptsDir, "sh")
	snc := StartTestAgent(t, config)

	runTestCheckExternalWrapped(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalUnixShellPathWithSpaces(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")
	holesDir := filepath.Join(testDir, "t", "scri pts")

	teardown := setupTeardown(t, holesDir)
	defer teardown()
	_ = copy.Copy(scriptsDir, holesDir)

	config := setupConfig(holesDir, "sh")
	snc := StartTestAgent(t, config)

	runTestCheckExternalDefault(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalWrappedUnixShellPathWithSpaces(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")
	holesDir := filepath.Join(testDir, "t", "scri pts")

	teardown := setupTeardown(t, holesDir)
	defer teardown()
	_ = copy.Copy(scriptsDir, holesDir)

	config := setupConfig(holesDir, "sh")
	snc := StartTestAgent(t, config)

	runTestCheckExternalWrapped(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalArgSpaces(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")
	holesDir := filepath.Join(testDir, "t", "scri pts")

	teardown := setupTeardown(t, holesDir)
	defer teardown()
	_ = copy.Copy(scriptsDir, holesDir)

	config := setupConfig(holesDir, "sh")
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_spaceargs", []string{""})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "", string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}
