//go:build !windows

package snclient

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckNSCWeb(t *testing.T) {
	testPort := getRandomFreeTCPPort(t)
	config := fmt.Sprintf(`
[/modules]
CheckBuiltinPlugins = enabled
WEBServer = enabled

[/settings/WEB/server]
port = %d
use ssl = false
password = test
	`, testPort)
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_nsc_web", []string{"-u", fmt.Sprintf("http://127.0.0.1:%d", testPort), "-p", "test"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(
		t,
		`^OK - REST API reachable`,
		string(res.BuildPluginOutput()),
		"output matches",
	)

	StopTestAgent(t, snc)
}
