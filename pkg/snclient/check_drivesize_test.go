//go:build !windows

package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckDrivesize(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_drivesize", []string{"warn=free > 0", "crit=free > 0", "drive=/"})
	assert.Equalf(t, CheckExitCritical, res.State, "state critical")
	assert.Regexpf(t,
		regexp.MustCompile(`^CRITICAL - / .*?\/.*? \(\d+\.\d+%\) \|'/ free'=.*?B;0;0;0;.*? '/ free %'=.*?%;0;0;0;100`),
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

	res = snc.RunCheck("check_drivesize", []string{"drive=k"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - failed to find disk partition", "output matches")

	// must not work, folder is not a mountpoint
	tmpFolder := t.TempDir()
	res = snc.RunCheck("check_drivesize", []string{"warn=inodes>100%", "crit=inodes>100%", "drive=" + tmpFolder})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN")
	assert.Contains(t, string(res.BuildPluginOutput()), `not mounted`, "output matches")

	// must work with folder argument instead of drive
	res = snc.RunCheck("check_drivesize", []string{"warn=inodes>100%", "crit=inodes>100%", "folder=" + tmpFolder})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), `OK - All 1 drive`, "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), `'`+tmpFolder+` inodes'=`, "output matches")

	StopTestAgent(t, snc)
}
