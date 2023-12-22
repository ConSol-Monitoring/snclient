//go:build !windows

package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckProcess(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_process", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t,
		regexp.MustCompile(`^OK: all \d+ processes are ok`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_process", []string{"process=noneexisting.exe"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
	assert.Equalf(t, "check_process failed to find anything with this filter. |'count'=0;;;0 'rss'=0B;;;0 'virtual'=0B;;;0",
		string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}
