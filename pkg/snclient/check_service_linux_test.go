package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckService(t *testing.T) {
	snc := Agent{}
	res := snc.RunCheck("check_service", []string{"filter='state=running'"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		regexp.MustCompile(`^OK: All \d+ service\(s\) are ok.`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_service", []string{"service=nonexistingservice"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state Unknown")
	assert.Containsf(t, string(res.BuildPluginOutput()), "UNKNOWN - could not find service: nonexistingservice", "output matches")
}
