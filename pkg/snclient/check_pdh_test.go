package snclient

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_PDH(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_pdh", []string{"counter=\\4\\30", "warn=value > 80", "crit=value > 90", "show-all"})
	assert.Equalf(t, CheckExitCritical, res.State, "The check could not be run successful")
	assert.Contains(t, string(res.BuildPluginOutput()), "CRITICAL")

	res = snc.RunCheck("check_pdh", []string{"counter=\\System\\System Up Time", "warn=value < 60", "crit=value < 30", "english", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "The check could not be run successful")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK")
	fmt.Printf("string(res.BuildPluginOutput()): %v\n", string(res.BuildPluginOutput()))
	t.Fail()
}

func Test_ExpandingWildCardPath(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_pdh", []string{"counter=\\4\\*", "expand-index", "instances", "warn=value > 80", "crit=value > 90"})
	assert.Equalf(t, CheckExitCritical, res.State, "The check could not be run")
	assert.Contains(t, string(res.BuildPluginOutput()), "CRITICAL")
}

func TestIndexLookup(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_pdh", []string{"counter=\\4\\30", "crit=value > 90", "show-all", "expand-index"})
	assert.Equalf(t, CheckExitCritical, res.State, "The check could not be run")
	assert.Contains(t, string(res.BuildPluginOutput()), "CRITICAL")
}
