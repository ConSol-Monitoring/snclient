//go:build windows

package snclient

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckWrapWindows(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := fmt.Sprintf(`
[/settings/external scripts]
allow arguments = true
timeout = 30

[/settings/external scripts/wrapped scripts]
test_wrapped = %s\wrapped.ps1
bat_wrapped = %s/check_dummy.bat

[/settings/external scripts/scripts]
check_dummy_bat = %s/check_dummy.bat
ps_command = powershell -noprofile -command %%ARGS%%
restart_process = cmd /c echo %s\restart_process.ps1 %%ARGS%%; exit($lastexitcode) | powershell.exe -command -

[/settings/external scripts/alias]
alias_ps_cpu = ps_command "ps | sort -des cpu"
`, scriptsDir, scriptsDir, scriptsDir, scriptsDir)

	snc := StartTestAgent(t, config)
	res := snc.RunCheck("test_wrapped", []string{"1", "test"})
	assert.Equalf(t, CheckExitWarning, res.State, "state Warning")
	assert.Equalf(t, "test", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("bat_wrapped", []string{"2", "test2"})
	assert.Equalf(t, CheckExitCritical, res.State, "state Critical")
	assert.Equalf(t, "test2", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_bat", []string{"2", "test3"})
	assert.Equalf(t, CheckExitCritical, res.State, "state Critical")
	assert.Equalf(t, "test3", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("restart_process", []string{"noneexistant"})
	assert.Equalf(t, CheckExitCritical, res.State, "state Critical")
	assert.Containsf(t, string(res.BuildPluginOutput()), "NET HELPMSG 2185", "output contains error message")

	res = snc.RunCheck("alias_ps_cpu", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Equalf(t, "test3", string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}
