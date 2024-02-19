package snclient

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckFiles(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_files", []string{})
	assert.Equalf(t, CheckExitUnknown, res.State, "state Unknown")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - no path specified")

	res = snc.RunCheck("check_files", []string{"path=."})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "files are ok")

	res = snc.RunCheck("check_files", []string{"path=.", "crit=count>10000", "max-depth=1"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "'count'=")

	res = snc.RunCheck("check_files", []string{"paths= ., t", "crit=count>10000", "max-depth=1"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "'count'=")

	res = snc.RunCheck("check_files", []string{"paths=noneex"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state Unknown")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - error walking directory noneex")

	res = snc.RunCheck("check_files", []string{"path=.", "filter=name eq 'check_files.go' and size gt 5K", "crit=count>0", "ok=count eq 0", "empty-state=ok"})
	assert.Equalf(t, CheckExitCritical, res.State, "state Critical")
	assert.Contains(t, string(res.BuildPluginOutput()), "'count'=")

	StopTestAgent(t, snc)
}

func TestCheckFilesNoPermission(t *testing.T) {
	snc := StartTestAgent(t, "")
	// prepare test folder
	tmpPath, err := os.MkdirTemp("", "testtmp*")
	require.NoError(t, err)

	for _, char := range []string{"a", "b", "c"} {
		err = os.WriteFile(filepath.Join(tmpPath, "file "+char+".txt"), []byte(strings.Repeat(char, 2000)), 0o600)
		require.NoErrorf(t, err, "writing %s worked")

		err = os.Mkdir(filepath.Join(tmpPath, "dir "+char), 0o700)
		require.NoErrorf(t, err, "writing %s worked")
	}
	err = os.Chmod(filepath.Join(tmpPath, "file b.txt"), 0o000)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(tmpPath, "dir b"), 0o000)
	require.NoError(t, err)

	res := snc.RunCheck("check_files", []string{"path=" + tmpPath, "filter=name eq 'file a.txt' and size gt 1K", "crit=count>0", "ok=count eq 0", "empty-state=ok"})
	assert.Equalf(t, CheckExitCritical, res.State, "state Critical")
	assert.Contains(t, string(res.BuildPluginOutput()), "'count'=")

	res = snc.RunCheck("check_files", []string{"path=" + tmpPath})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - All 3 files are ok: (5.86 KiB) |'count'=3;;;0 'size'=6000B;;;0")

	defer os.RemoveAll(tmpPath)

	StopTestAgent(t, snc)
}
