package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_PDH(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_pdh", []string{"counter=\\Prozessorinformationen(0,1)\\% Leistungslimit", "warn=value > 80", "crit=value > 90"})
	assert.Equalf(t, CheckExitWarning, res.State, "The check could not be run successful")
	assert.Contains(t, string(res.BuildPluginOutput()), "WARNING")

	res = snc.RunCheck("check_pdh", []string{"counter=\\Arbeitsspeicher\\Zusagegrenze", "warn=value > 80", "crit=value > 90"})
	assert.Equalf(t, CheckExitCritical, res.State, "The check could not be run successful")
	assert.Contains(t, string(res.BuildPluginOutput()), "CRITICAL")

	res = snc.RunCheck("check_pdh", []string{"counter=\\System\\System Up Time", "warn=value < 300", "crit=value < 30", "english", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "The check could not be run successful")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK")
}

func Test_ExpandingWildCardPath(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_pdh", []string{"counter=\\4\\*", "expand-index", "instances", "warn=value > 80", "crit=value > 90"})
	assert.Equalf(t, CheckExitOK, res.State, "The check could not be run")
	assert.Contains(t, string(res.BuildPluginOutput()), "89")
}

func TestIndexLookup(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_pdh", []string{"counter=\\4\\30", "crit=value > 90", "show-all", "expand-index"})
	assert.Equalf(t, CheckExitCritical, res.State, "The check could not be run")
	assert.Contains(t, string(res.BuildPluginOutput()), "CRITICAL")
}
