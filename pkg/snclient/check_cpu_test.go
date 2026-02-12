package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckCPU(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_cpu", []string{"warn=load = 101", "crit=load = 102"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		`^OK - CPU load is ok. \d+% on \d+ cores \|'total 5m'=\d+%;101;102 'total 1m'=\d+%;101;102 'total 5s'=\d+%;101;102$`,
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_cpu", []string{"warn=load = 101", "crit=load = 102", "time=3m", "time=7m"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "total 3m", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "total 7m", "output matches")

	res = snc.RunCheck("check_cpu", []string{"warn=load = 101", "crit=load = 102", "filter=none"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "core0 1m", "output matches")

	res = snc.RunCheck("check_cpu", []string{"warn=load = 101", "crit=load = 102", "filter=core=core0"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "core0 1m", "output matches")

	res = snc.RunCheck("check_cpu", []string{"warn=load = 101", "crit=load = 102", "filter=core_id=core0"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "core0 1m", "output matches")
	assert.NotContainsf(t, string(res.BuildPluginOutput()), "total 1m", "output matches not")

	res = snc.RunCheck("check_cpu", []string{"warn=load = 101", "crit=load = 102", "filter=core_id != core1"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "core0 1m", "output matches")
	assert.NotContainsf(t, string(res.BuildPluginOutput()), "core1 1m", "output matches not")

	res = snc.RunCheck("check_cpu", []string{"warn=load = 101", "crit=load = 102", "-n", "10", "--hide-args=false"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "RSS", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "%MEM", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "COMMAND", "output matches")

	StopTestAgent(t, snc)
}
