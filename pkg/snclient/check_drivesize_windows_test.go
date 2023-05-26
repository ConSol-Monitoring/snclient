package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckDrivesize(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_drivesize", []string{"warn=free > 0", "crit=free > 0", "drive=c"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		regexp.MustCompile(`^OK All 1 drive\(s\) are ok \|'c: free'=.*?B;0;0;0;.*? 'c: free %'=\d+%;0;0;0;100`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_drivesize", []string{"filter=free<0"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK: No drives found", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=free>0", "total"})
	assert.Equalf(t, CheckExitWarning, res.State, "state WARNING")
	assert.Contains(t, string(res.BuildPluginOutput()), "c: used %", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "total free", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "c: free", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), ";0;90;0;100", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"drive=k"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - Failed to find disk partition", "output matches")

	res = snc.RunCheck("check_drivesize", []string{
		"warning=used > 99",
		"crit=used > 99.5",
		"empty-state=unknown",
		`filter=type in ('fixed') AND mounted=1 AND name not like '\?\'`,
		"show-all",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK: No drives found", "output matches")

	StopTestAgent(t, snc)
}
