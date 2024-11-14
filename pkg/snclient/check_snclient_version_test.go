package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckSNClientVersion(t *testing.T) {
	snc := Agent{}
	res := snc.RunCheck("check_snclient_version", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		`^SNClient\+ v\d+`,
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_nscp_version", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		`^SNClient\+ v\d+`,
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_snclient_version", []string{"warn='version > 0.1'"})
	assert.Equalf(t, CheckExitWarning, res.State, "state Warning")
	assert.Containsf(t, string(res.BuildPluginOutput()), ";0.1", "output matches")
}
