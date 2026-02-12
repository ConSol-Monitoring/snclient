package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckCPUUtilization(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_cpu_utilization", []string{"warn=none", "crit=none", "range=1m"})
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - user:", "output matches")

	res = snc.RunCheck("check_cpu_utilization", []string{"warn=none", "crit=none", "range=1m", "-n", "10", "--show-args"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "RSS", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "%MEM", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "COMMAND", "output matches")

	StopTestAgent(t, snc)
}
