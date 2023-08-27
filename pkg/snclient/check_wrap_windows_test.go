//go:build windows

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
script= %s

[/settings/external scripts/scripts]
check_doesnotexist = /a/path/that/does/not/exist/nonexisting_script "no" "no"

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

[/settings/external scripts/wrapped scripts]
check_dummy_wrapped_noparm = check_dummy.EXTENSION
check_dummy_wrapped = check_dummy.EXTENSION $ARG1$ "$ARG2$"
check_dummy_wrapped_ok = check_dummy.EXTENSION 0 "i am ok wrapped"
check_dummy_wrapped_critical = check_dummy.EXTENSION 2 "i am critical wrapped"

[/settings/external scripts/scripts/timeoutscript]
timeout = 1
command = ping 127.0.0.1 -n 10
`, scriptsDir)
	config = strings.ReplaceAll(config, "EXTENSION", scriptsType)

	return config
}

func TestMain(m *testing.M) {
	cmd := exec.Command("go", "build", "-o", "check_dummy.exe", "check_dummy.go")
	cmd.Dir = "t/scripts"
	_, _ = cmd.CombinedOutput()

	// run the tests
	exitCode := m.Run()

	// run teardown code
	_ = os.Remove("t/scripts/check_dummy.exe")

	// exit with the same exit code as the tests
	os.Exit(exitCode)
}

func TestCheckExternalWindowsTimeout(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(scriptsDir, "bat")
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("timeoutscript", []string{})
	assert.Equalf(t, CheckExitUnknown, res.State, "state matches")
	assert.Equalf(t, "UNKNOWN: script run into timeout after 1s\n", string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}

func TestCheckExternalWindowsBat(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(scriptsDir, "bat")
	snc := StartTestAgent(t, config)

	runTestCheckExternalDefault(t, snc)
	runTestCheckExternalArgs(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalWrappedWindowsBat(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(scriptsDir, "bat")
	snc := StartTestAgent(t, config)

	runTestCheckExternalWrapped(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalWindowsBatPathWithSpaces(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")
	holesDir := filepath.Join(testDir, "t", "scri pts")

	teardown := setupTeardown(t, holesDir)
	defer teardown()
	_ = copy.Copy(scriptsDir, holesDir)

	config := setupConfig(holesDir, "bat")
	snc := StartTestAgent(t, config)

	runTestCheckExternalDefault(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalWrappedWindowsBatPathWithSpaces(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")
	holesDir := filepath.Join(testDir, "t", "scri pts")

	teardown := setupTeardown(t, holesDir)
	defer teardown()
	_ = copy.Copy(scriptsDir, holesDir)

	config := setupConfig(holesDir, "bat")
	snc := StartTestAgent(t, config)

	runTestCheckExternalWrapped(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalWindowsPs(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(scriptsDir, "bat")
	snc := StartTestAgent(t, config)

	runTestCheckExternalDefault(t, snc)
	runTestCheckExternalArgs(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalWrappedWindowsPs(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(scriptsDir, "bat")
	snc := StartTestAgent(t, config)

	runTestCheckExternalWrapped(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalWindowsPsPathWithSpaces(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")
	holesDir := filepath.Join(testDir, "t", "scri pts")

	teardown := setupTeardown(t, holesDir)
	defer teardown()
	_ = copy.Copy(scriptsDir, holesDir)

	config := setupConfig(holesDir, "bat")
	snc := StartTestAgent(t, config)

	runTestCheckExternalDefault(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalWrappedWindowsPsPathWithSpaces(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")
	holesDir := filepath.Join(testDir, "t", "scri pts")

	teardown := setupTeardown(t, holesDir)
	defer teardown()
	_ = copy.Copy(scriptsDir, holesDir)

	config := setupConfig(holesDir, "bat")
	snc := StartTestAgent(t, config)

	runTestCheckExternalWrapped(t, snc)

	StopTestAgent(t, snc)
}
