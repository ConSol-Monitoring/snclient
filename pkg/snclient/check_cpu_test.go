package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckCPU(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_cpu", []string{"warn=load = 101", "crit=load = 102"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		regexp.MustCompile(`^OK: CPU load is ok. \|'total 5m'=\d+%;101;102 'total 1m'=\d+%;101;102 'total 5s'=\d+%;101;102$`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_cpu", []string{"warn=load = 101", "crit=load = 102", "time=3m", "time=7m"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "total 3m", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "total 7m", "output matches")

	StopTestAgent(t, snc)
}
