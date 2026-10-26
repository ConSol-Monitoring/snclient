package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckCPU(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_cpu", []string{"warn=load = 101", "crit=load = 102"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(
		t,
		`^OK - CPU load is ok. \d+% on \d+ cores \|'total 5s'=\d+[\.\d]*%;101;102 'total 1m'=\d+[\.\d]*%;101;102 'total 5m'=\d+[\.\d]*%;101;102$`,
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

	res = snc.RunCheck("check_cpu", []string{"warn=load = 101", "crit=load = 102", "-n", "10", "--show-args"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "RSS", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "%MEM", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "COMMAND", "output matches")

	StopTestAgent(t, snc)
}

func TestSortedCpuKeys(t *testing.T) {
	assert.Equal(
		t,
		[]string{"total", "core0", "core1", "core2", "core10", "core22"},
		sortedCPUKeys([]string{"total", "core22", "core10", "core2", "core1", "core0"}),
		"total first, cores sorted numerically",
	)

	assert.Equal(
		t,
		[]string{"7s", "1m", "5m"},
		sortedCPUTimes([]string{"5m", "1m", "7s"}),
		"times sorted by duration ascending",
	)
}
