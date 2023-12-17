package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckLoad(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_load", []string{"warn=load >= 0", "crit=load >= 0"})
	assert.Equalf(t, CheckExitCritical, res.State, "state Critical")
	assert.Regexpf(t,
		regexp.MustCompile(`^CRITICAL: total load average: [\d\.]+, [\d\.]+, [\d\.]+ on \d+ cores \|'load1'=[\d\.]+;0;0;0 'load5'=[\d\.]+;0;0;0 'load15'=[\d\.]+;0;0;0$`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_load", []string{"-w", "0,0,0", "-c", "999,998,997"})
	assert.Regexpf(t,
		regexp.MustCompile(`total load average: [\d\.]+, [\d\.]+, [\d\.]+ on \d+ cores \|'load1'=[\d\.]+;0;999;0 'load5'=[\d\.]+;0;998;0 'load15'=[\d\.]+;0;997;0$`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	StopTestAgent(t, snc)
}
