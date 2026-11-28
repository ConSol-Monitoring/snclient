//go:build !windows

package snclient

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckNSCWeb(t *testing.T) {
	config := `
[/modules]
CheckBuiltinPlugins = enabled
WEBServer = enabled

[/settings/WEB/server]
port = 0
use ssl = false
password = test
	`
	snc := StartTestAgent(t, config)
	testPort := testListenerPort(t, snc)

	res := snc.RunCheck("check_snclient", []string{"-u", fmt.Sprintf("http://127.0.0.1:%d", testPort), "-p", "test"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(
		t,
		`^OK - REST API reachable`,
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_nsc_web", []string{"-u", fmt.Sprintf("http://127.0.0.1:%d", testPort), "-p", "test"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(
		t,
		`^OK - REST API reachable`,
		string(res.BuildPluginOutput()),
		"output matches",
	)

	StopTestAgent(t, snc)
}
