//go:build !windows

package snclient

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckDrivesize(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_drivesize", []string{"warn=free > 0", "crit=free > 0", "drive=/"})
	assert.Equalf(t, CheckExitCritical, res.State, "state critical")
	assert.Regexpf(
		t,
		`^CRITICAL - / .*?\/.*? \(\d+\.\d+%\) \|'/ free'=.*?B;0;0;0;.*? '/ free %'=.*?%;0;0;0;100`,
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_drivesize", []string{"warn=free_bytes > 0", "crit=free_bytes > 0", "drive=/"})
	assert.Equalf(t, CheckExitCritical, res.State, "state critical")
	assert.Regexpf(
		t,
		`^CRITICAL - / .*?\/.*? \(\d+\.\d+%\) \|'/ free'=.*?B;0;0;0;.*? '/ free %'=.*?%;0;0;0;100`,
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_drivesize", []string{"filter=free<0", "empty-state=0"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - No drives found", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=free>0", "total", "filter=none"})
	assert.Contains(t, string(res.BuildPluginOutput()), "/ used %", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "total free", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "/ free", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), ";0;;0;100", "output matches")

	// must not work, folder is not a mountpoint
	tmpFolder := t.TempDir()
	res = snc.RunCheck("check_drivesize", []string{"warn=inodes>100%", "crit=inodes>100%", "drive=" + tmpFolder})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN")
	assert.Contains(t, string(res.BuildPluginOutput()), `not mounted`, "output matches")

	// must work with folder argument instead of drive
	res = snc.RunCheck("check_drivesize", []string{"warn=inodes>100%", "crit=inodes>100%", "folder=" + tmpFolder})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), `OK - All 1 drive`, "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), `'`+tmpFolder+` inodes_used %'=`, "output matches")

	StopTestAgent(t, snc)
}

func TestNonexistingDrive(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_drivesize", []string{"drive=/dev/sdxyz999"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - No drives found", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"drive=/dev/sdxyz999", "empty-state=ok"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - No drives found", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"drive=/dev/sdxyz999", "empty-state=warn"})
	assert.Equalf(t, CheckExitWarning, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "WARNING - No drives found", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"drive=/dev/sdxyz999", "empty-state=warning"})
	assert.Equalf(t, CheckExitWarning, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "WARNING - No drives found", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"drive=/dev/sdxyz999", "empty-state=1"})
	assert.Equalf(t, CheckExitWarning, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "WARNING - No drives found", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"drive=/dev/sdxyz999", "empty-state=crit"})
	assert.Equalf(t, CheckExitCritical, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "CRITICAL - No drives found", "output matches")

	StopTestAgent(t, snc)
}

func TestRootDriveThresholdsDrivePercentSign(t *testing.T) {
	snc := StartTestAgent(t, "")

	// This tests the conditions using '[drive] used %' keyword
	res := snc.RunCheck("check_drivesize", []string{"drive=/", "warn='/ used %' >= 0", "crit=none"})
	assert.Equalf(t, CheckExitWarning, res.State, "state should be WARNING")

	res = snc.RunCheck("check_drivesize", []string{"drive=/", "crit='/ used %' >= 0"})
	assert.Equalf(t, CheckExitCritical, res.State, "state should be CRITICAL")

	res = snc.RunCheck("check_drivesize", []string{"drive=/", "warn='/ used %' >= 0", "crit='/ used %' >= 0"})
	assert.Equalf(t, CheckExitCritical, res.State, "state should be CRITICAL")

	res = snc.RunCheck("check_drivesize", []string{"drive=/", "warn='/ used %' >= 0", "crit='/ used %' >= 0", "ok='/ used %' >= 0"})
	assert.Equalf(t, CheckExitOK, res.State, "state should be CRITICAL")

	StopTestAgent(t, snc)
}

func TestRootDriveThresholdsUsedPct(t *testing.T) {
	snc := StartTestAgent(t, "")

	// This tests the conditions using '[drive] used_pct' keyword
	res := snc.RunCheck("check_drivesize", []string{"drive=/", "warn='/ used_pct' >= 0", "crit=none"})
	assert.Equalf(t, CheckExitWarning, res.State, "state should be WARNING")

	res = snc.RunCheck("check_drivesize", []string{"drive=/", "crit='/ used_pct' >= 0"})
	assert.Equalf(t, CheckExitCritical, res.State, "state should be CRITICAL")

	res = snc.RunCheck("check_drivesize", []string{"drive=/", "warn='/ used_pct' >= 0", "crit='/ used_pct' >= 0"})
	assert.Equalf(t, CheckExitCritical, res.State, "state should be CRITICAL")

	res = snc.RunCheck("check_drivesize", []string{"drive=/", "warn='/ used_pct' >= 0", "crit='/ used_pct' >= 0", "ok='/ used_pct' >= 0"})
	assert.Equalf(t, CheckExitOK, res.State, "state should be CRITICAL")

	StopTestAgent(t, snc)
}

func TestRootDriveThresholdsUsedPercentSign(t *testing.T) {
	snc := StartTestAgent(t, "")

	// Generic 'used %' keyword, should trigger
	res := snc.RunCheck("check_drivesize", []string{"drive=/", "warning='used %' gt 0", "critical=none"})
	assert.Equalf(t, CheckExitWarning, res.State, "state should be WARNING")

	res = snc.RunCheck("check_drivesize", []string{"drive=/", "critical='used %' gt 0"})
	assert.Equalf(t, CheckExitCritical, res.State, "state should be CRITICAL")

	StopTestAgent(t, snc)
}

func TestRootDriveSpecializedDriveConditions(t *testing.T) {
	snc := StartTestAgent(t, "")

	// In critical conditions, the second one is filtered as it contains the drive="/" keyword.
	// Since such a condition is present, other critical conditions are filtered out for that entry
	res := snc.RunCheck("check_drivesize", []string{"drive=/", "critical='used %' gt 0", "critical=drive eq '/' and 'used_pct' gt 100"})
	assert.Equalf(t, CheckExitOK, res.State, "state should be OK")

	// Filtering does not work here, since the specialized condition is in the warning threshold, not critical threshold
	res = snc.RunCheck("check_drivesize", []string{"drive=/", "critical='used %' gt 0", "warning=drive eq '/' and 'used_pct' gt 0"})
	assert.Equalf(t, CheckExitCritical, res.State, "state should be WARNING")

	StopTestAgent(t, snc)
}

func TestRootDriveSpecializedDriveConditionsPerfdata(t *testing.T) {
	switch runtime.GOOS {
	case "linux":
		snc := StartTestAgent(t, "")

		// Check if the perfdata is printed out according to the specialized conditions
		res := snc.RunCheck("check_drivesize", []string{"drive=/", "drive=/tmp", "critical='used_pct' gt 99", "critical=drive eq '/tmp' and 'used_pct' gt 1", "warn=none"})
		assert.Equalf(t, CheckExitCritical, res.State, "state should be CRITICAL")
		assert.Regexp(t, `'/ used %'=[^;]+;[^;]*;99;0;100`, string(res.BuildPluginOutput()), `'/tmp used %' perfdata matches`)
		assert.Regexp(t, `'/tmp used %'=[^;]+;[^;]*;1;0;100`, string(res.BuildPluginOutput()), `'/tmp used %' perfdata matches`)

		StopTestAgent(t, snc)
	default:
		t.Skipf("skipping tests due to platform: %s", runtime.GOOS)
	}
}
