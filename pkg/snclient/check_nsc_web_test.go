//go:build !windows

package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckNSCWeb(t *testing.T) {
	config := `
[/modules]
CheckBuiltinPlugins = enabled
WEBServer = enabled

[/settings/WEB/server]
port = 45666
use ssl = false
password = test
	`
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_nsc_web", []string{"-u", "http://127.0.0.1:45666", "-p", "test"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t,
		regexp.MustCompile(`^OK - REST API reachable`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	StopTestAgent(t, snc)
}
