package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckEventlog(t *testing.T) {
	snc := Agent{}
	// match nothing for now...
	res := snc.RunCheck("check_eventlog", []string{"filter=id < 0"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		regexp.MustCompile(`^OK - No entries found`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
}
