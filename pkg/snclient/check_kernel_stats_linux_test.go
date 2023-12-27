package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKernelStats(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_kernel_stats", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t,
		regexp.MustCompile(`^OK - Context Switches [\d.]+/s, Process Creations [\d.]+/s`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	StopTestAgent(t, snc)
}
