// run -gcflags=-d=checkptr
//go:build windows
// +build windows

package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckPDH(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_pdh", []string{`counter=\4\30`, "warn=value > 80", "crit=value > 90"})
	assert.Equalf(t, CheckExitCritical, res.State, "The check could not be run successful")
	assert.Contains(t, string(res.BuildPluginOutput()), "CRITICAL")

	res = snc.RunCheck("check_pdh", []string{`counter=\System\System Up Time`, "warn=value < 60", "crit=value < 30", "english", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "The check could not be run successful")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK")

	StopTestAgent(t, snc)
}

func TestCheckPDHOptionalAlias(t *testing.T) {
	// svchost.exe should run on all windows instances
	snc := StartTestAgent(t, "")
	res := snc.RunCheck("check_pdh", []string{`counter:svchost=\Process(svchost)\Private Bytes`, "warn=value < 200", "crit=value < 500", "instances", "english"})
	assert.Equalf(t, CheckExitOK, res.State, "The check could not be run successful")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK")
	StopTestAgent(t, snc)
}

func TestCheckPDHExpandingWildCardPath(t *testing.T) {
	snc := StartTestAgent(t, "")
	res := snc.RunCheck("check_pdh", []string{`counter=\4\*`, "expand-index", "instances", "warn=count < 5", "crit=count < 10", "english"})
	assert.Equalf(t, CheckExitCritical, res.State, "The check could not be run")
	assert.Contains(t, string(res.BuildPluginOutput()), "CRITICAL")

	StopTestAgent(t, snc)
}

func TestCheckPDHIndexLookup(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_pdh", []string{`counter=\4\30`, "crit=value > 90", "show-all", "expand-index"})
	assert.Equalf(t, CheckExitCritical, res.State, "The check could not be run")
	assert.Contains(t, string(res.BuildPluginOutput()), "CRITICAL")

	StopTestAgent(t, snc)
}
