package snclient

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckTCP(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { LogError(listener.Close()) })
	host, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	snc := StartTestAgent(t, `
[/modules]
CheckBuiltinPlugins = enabled
`)
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_tcp", []string{"-H", host, "-p", port})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(
		t,
		fmt.Sprintf(`^TCP OK - [\d.]+ seconds response time on 127\.0\.0\.1 port %s\b`, port),
		string(res.BuildPluginOutput()),
		"output matches",
	)
}
