package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckUnknown(t *testing.T) {
	snc := Agent{}
	res := snc.RunCheck("not_there", []string{})
	assert.Equalf(t, CheckExitUnknown, res.State, "state Unknown")
	assert.Regexpf(t,
		regexp.MustCompile(`^UNKNOWN - No such check: not_there`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
}
