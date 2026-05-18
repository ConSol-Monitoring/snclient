package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckSSH(t *testing.T) {
	config := `
[/modules]
CheckBuiltinPlugins = enabled
`
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_ssh", []string{"-H", "github.com", "-p", "22"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(
		t,
		`^SSH OK - [\d.]+ seconds response time on github.com port 22`,
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_ssh", []string{"-H", "bitbucket.org"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(
		t,
		`^SSH OK - [\d.]+ seconds response time on bitbucket.org port 22`,
		string(res.BuildPluginOutput()),
		"output matches",
	)

	StopTestAgent(t, snc)
}
