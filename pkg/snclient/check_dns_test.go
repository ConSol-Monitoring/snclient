//go:build !windows

package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckDNS(t *testing.T) {
	config := `
[/modules]
CheckBuiltinPlugins = enabled
	`
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t,
		`^OK - labs\.consol\.de returns 94\.185\.89\.33`,
		string(res.BuildPluginOutput()),
		"output matches",
	)

	StopTestAgent(t, snc)
}
