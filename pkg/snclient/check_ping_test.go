package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckPing(t *testing.T) {
	config := `
[/modules]
CheckBuiltinPlugins = enabled
`
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_ping", []string{"host=127.0.0.1", "count=1", "timeout=5000"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t,
		regexp.MustCompile(`^OK - [\d.]+ seconds response time on port 45666`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	StopTestAgent(t, snc)
}
