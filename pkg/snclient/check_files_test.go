package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
