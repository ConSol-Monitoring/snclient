package snclient

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMount(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_mount", []string{"mount=/not_there", "options=rw,relatime"})
	assert.Equalf(t, CheckExitCritical, res.State, "state Critical")
	assert.Contains(t, string(res.BuildPluginOutput()), "CRITICAL - mount /not_there not mounted", "output matches")

	if runtime.GOOS == "windows" {
		res = snc.RunCheck("check_mount", []string{"mount=C:"})
	} else {
		res = snc.RunCheck("check_mount", []string{"mount=/"})
	}
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - 1 mount(s) found", "output matches")

	inv, err := snc.getInventoryEntry(t.Context(), "check_mount")
	require.NoError(t, err)
	require.NotEmptyf(t, inv, "expected mounts list to be non-empty")
	res = snc.RunCheck("check_mount", []string{"mount=" + inv[0]["mount"], "options=" + inv[0]["options"], "fstype=" + inv[0]["fstype"]})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - 1 mount(s) found", "output matches")

	StopTestAgent(t, snc)
}

func TestMountNoMountArgument(t *testing.T) {
	snc := StartTestAgent(t, "")

	// mount= left empty means all mounts are checked
	res := snc.RunCheck("check_mount", []string{})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN")
	assert.Equalf(t, "UNKNOWN - must specify at least one of mount/options/fstype", string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}

func TestMountMultipleMounts(t *testing.T) {
	snc := StartTestAgent(t, "")

	inv, err := snc.getInventoryEntry(t.Context(), "check_mount")
	require.NoError(t, err)
	require.NotEmptyf(t, inv, "expected mounts list to be non-empty")

	realMounts := []string{}
	for _, entry := range inv {
		if entry["mount"] != "" {
			realMounts = append(realMounts, entry["mount"])
		}
	}
	require.NotEmptyf(t, realMounts, "expected at least one mount")

	// checking multiple existing mounts at once must be ok
	args := make([]string, 0, min(len(realMounts), 3)+1)
	for _, mount := range realMounts[:min(len(realMounts), 3)] {
		args = append(args, "mount="+mount)
	}
	res := snc.RunCheck("check_mount", args)
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexp(t, `OK - [\d]+ mount\(s\) found`, string(res.BuildPluginOutput()), "output matches")

	// if one of them is missing, the whole check must raise critical
	missing := "not_mounted_xyz"
	if runtime.GOOS == "windows" {
		missing = `\\?\Volume{ffffffff-ffff-ffff-ffff-ffffffffffff}`
	}
	res = snc.RunCheck("check_mount", append(args, "mount="+missing))
	assert.Equalf(t, CheckExitCritical, res.State, "state Critical")
	assert.Contains(t, string(res.BuildPluginOutput()), "mount "+missing+" not mounted", "output matches")

	StopTestAgent(t, snc)
}

func TestMountSingleSpecifiedMount(t *testing.T) {
	snc := StartTestAgent(t, "")

	if runtime.GOOS == "windows" {
		res := snc.RunCheck("check_mount", []string{"mount=C:\\"})
		assert.Equalf(t, CheckExitOK, res.State, "state OK")
		assert.Contains(t, string(res.BuildPluginOutput()), "OK - 1 mount(s) found", "output matches")
	} else {
		res := snc.RunCheck("check_mount", []string{"mount=/"})
		assert.Equalf(t, CheckExitOK, res.State, "state OK")
		assert.Contains(t, string(res.BuildPluginOutput()), "OK - 1 mount(s) found", "output matches")
	}

	StopTestAgent(t, snc)
}
