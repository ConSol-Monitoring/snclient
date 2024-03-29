package snclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMount(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_mount", []string{"mount=/"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - mounts are as expected", "output matches")

	res = snc.RunCheck("check_mount", []string{"mount=/not_there", "options=rw,relatime"})
	assert.Equalf(t, CheckExitCritical, res.State, "state Critical")
	assert.Contains(t, string(res.BuildPluginOutput()), "CRITICAL - mount /not_there not mounted", "output matches")

	inv, err := snc.getInventory(context.TODO(), "check_mount")
	require.NoError(t, err)
	require.NotEmptyf(t, inv, "expected mounts list to be non-empty")
	res = snc.RunCheck("check_mount", []string{"mount=" + inv[0]["mount"], "options=" + inv[0]["options"], "fstype=" + inv[0]["fstype"]})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - mounts are as expected", "output matches")

	StopTestAgent(t, snc)
}
