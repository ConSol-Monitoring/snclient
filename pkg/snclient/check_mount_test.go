package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMount(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_mount", []string{"mount=/not_there", "options=rw,relatime"})
	assert.Equalf(t, CheckExitCritical, res.State, "state Critical")
	assert.Contains(t, string(res.BuildPluginOutput()), "CRITICAL - mount /not_there not mounted", "output matches")

	StopTestAgent(t, snc)
}
