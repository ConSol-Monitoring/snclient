//go:build !windows

package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckProcess(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_process", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state critical")
	assert.Regexpf(t,
		regexp.MustCompile(`^OK: all processes are ok`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	StopTestAgent(t, snc)
}
