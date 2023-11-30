package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckDrivesize(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_drivesize", []string{"warn=free > 0", "crit=free > 0", "drive=c"})
	assert.Equalf(t, CheckExitCritical, res.State, "state critical")
	assert.Regexpf(t,
		regexp.MustCompile(`^CRITICAL: c: .*?\/.*? \(\d+\.\d+%\) \|'c: free'=.*?B;0;0;0;.*? 'c: free %'=.*?%;0;0;0;100`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_drivesize", []string{"filter=free<0", "empty-state=0"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK: No drives found", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=free>0", "total"})
	assert.Contains(t, string(res.BuildPluginOutput()), "C:\\ used %", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "total free", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "C:\\ free", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), ";0;;0;100", "output matches")

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
	assert.Contains(t, string(res.BuildPluginOutput()), "OK: C:\\ ", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), ";99;99.5;0;100", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "drive=c:\\Windows"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), `OK: All 1 drive`, "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), `c:\Windows used %`, "output matches")

	StopTestAgent(t, snc)
}
