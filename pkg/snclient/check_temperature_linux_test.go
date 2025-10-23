package snclient

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTemperature(t *testing.T) {
	snc := StartTestAgent(t, "")
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_temperature", []string{})
	if res.State == CheckExitUnknown && strings.Contains(string(res.BuildPluginOutput()), "failed to find any sensors") {
		// no sensors found, cannot test
		return
	}
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t,
		`^OK -.*(core_0|amdgpu_edge): [\d.]+ °C`,
		string(res.BuildPluginOutput()),
		"output matches",
	)
}
