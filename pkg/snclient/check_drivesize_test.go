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
		regexp.MustCompile(`^CRITICAL: / .*?\/.*? used \|'/ free'=.*?B;0;0;0;.*? '/ free %'=.*?%;0;0;0;100`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_drivesize", []string{"filter=free<0"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK: No drives found", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=free>0", "total"})
	assert.Contains(t, string(res.BuildPluginOutput()), "/ used %", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "total free", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "/ free", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), ";0;;0;100", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"drive=k"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - Failed to find disk partition", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "drive=/tmp/"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), `OK: All 1 drive`, "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), `/tmp/ used %`, "output matches")

	StopTestAgent(t, snc)
}
