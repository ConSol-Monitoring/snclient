package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckEventlog(t *testing.T) {
	snc := Agent{}
	res := snc.RunCheck("check_eventlog", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		regexp.MustCompile(`^OK: Event log seems fine`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
}
