//go:build linux || windows || darwin

package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckConnections(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_connections", []string{"warn=established >= 10000", "crit=established >= 20000"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - total connections: ", "output matches")

	StopTestAgent(t, snc)
}
