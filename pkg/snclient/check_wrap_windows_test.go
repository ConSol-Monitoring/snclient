//go:build windows

package snclient

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
)

func TestCheckExternalWindowsBat(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(t, scriptsDir, "bat")
	snc := StartTestAgent(t, config)

	runTestCheckExternalDefault(t, snc)
	runTestCheckExternalArgs(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalWrappedWindowsBat(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(t, scriptsDir, "bat")
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

	config := setupConfig(t, holesDir, "bat")
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

	config := setupConfig(t, holesDir, "bat")
	snc := StartTestAgent(t, config)

	runTestCheckExternalWrapped(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalWindowsPs(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(t, scriptsDir, "ps1")
	snc := StartTestAgent(t, config)

	runTestCheckExternalDefault(t, snc)
	runTestCheckExternalArgs(t, snc)

	res := snc.RunCheck("check_win_none_ex", []string{})
	assert.Equalf(t, CheckExitUnknown, res.State, "state matches")
	assert.Containsf(t, string(res.BuildPluginOutput()), "timeout.ps1", "output matches")

	StopTestAgent(t, snc)
}

func TestCheckExternalWrappedWindowsPs(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(t, scriptsDir, "ps1")
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

	config := setupConfig(t, holesDir, "ps1")
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

	config := setupConfig(t, holesDir, "ps1")
	snc := StartTestAgent(t, config)

	runTestCheckExternalWrapped(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalWindowsPS1InSubdir(t *testing.T) {
	testDir, _ := os.Getwd()

	extConfig := `
[/settings/external scripts]
allow arguments = true
allow nasty characters = true
`
	config := setupConfig(t, testDir, "ps1")
	snc := StartTestAgent(t, config+extConfig)

	res := snc.RunCheck("check_win_subargs", []string{"-state 1 -message 'output 123'"})
	assert.Equalf(t, CheckExitWarning, res.State, "state matches")
	assert.Equalf(t, "output 123", string(res.BuildPluginOutput()), "output matches")

	// catch internal powershell errors
	res = snc.RunCheck("check_win_subargs", []string{"-state xyz -message 'output 123'"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state matches")
	assert.Containsf(t, string(res.BuildPluginOutput()), "Cannot convert value", "output matches")

	StopTestAgent(t, snc)
}
