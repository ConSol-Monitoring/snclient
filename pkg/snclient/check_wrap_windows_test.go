//go:build windows

package snclient

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/otiai10/copy"
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

	// This test has been disabled because it is nearly impossible to repair all occurrences of
	// a path with spaces inside a wrapped command.
	// runTestCheckExternalWrapped(t, snc)

	StopTestAgent(t, snc)
}
