//go:build !windows

package snclient

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckHTTP(t *testing.T) {
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

	res := snc.RunCheck("check_http", []string{"-H", "localhost", "-p", fmt.Sprintf("%d", testPort), "-u", "/index.html"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(
		t,
		`^HTTP OK - HTTP/1.1 200 OK`,
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_http", []string{"-H", "localhost", "-p", fmt.Sprintf("%d", testPort), "-u", "/api/v1/inventory"})
	assert.Equalf(t, CheckExitWarning, res.State, "state warning")
	assert.Containsf(t, string(res.BuildPluginOutput()), "HTTP/1.1 403 Forbidden", "output matches")

	StopTestAgent(t, snc)
}
