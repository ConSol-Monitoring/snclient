//go:build !windows

package snclient

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
)

func TestCheckExternalUnixNonExist(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(t, scriptsDir, "sh")
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_doesnotexist", []string{})
	assert.Equalf(t, CheckExitCritical, res.State, "state matches")
	assert.Regexp(t, "CRITICAL - Return code of 127.*actually exists", string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}

func TestCheckExternalUnixExeInSubdir(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(t, scriptsDir, "exe")
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_dummy_subdir", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Regexp(t, "OK: i am ok in my subdir", string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}

func TestCheckExternalUnixShell(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(t, scriptsDir, "sh")
	snc := StartTestAgent(t, config)

	runTestCheckExternalDefault(t, snc)
	runTestCheckExternalArgs(t, snc)

	StopTestAgent(t, snc)
}

func TestCheckExternalWrappedUnixShell(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := setupConfig(t, scriptsDir, "sh")
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

	config := setupConfig(t, holesDir, "sh")
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

	config := setupConfig(t, holesDir, "sh")
	snc := StartTestAgent(t, config)

	runTestCheckExternalWrapped(t, snc)

	StopTestAgent(t, snc)
}
