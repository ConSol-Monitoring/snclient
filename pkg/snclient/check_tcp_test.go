package snclient

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckTCP(t *testing.T) {
	testPort := getRandomFreeTCPPort(t)
	config := fmt.Sprintf(`
[/modules]
CheckBuiltinPlugins = enabled
WEBServer = enabled

[/settings/WEB/server]
port = %d
use ssl = false
`, testPort)
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_tcp", []string{"-H", "localhost", "-p", fmt.Sprintf("%d", testPort)})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(
		t,
		fmt.Sprintf(`^TCP OK - [\d.]+ seconds response time on localhost port %d`, testPort),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	StopTestAgent(t, snc)
}
