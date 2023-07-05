package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckWMI(t *testing.T) {
	config := `
	[/modules]
	CheckWMI = enabled
	`
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_wmi", []string{"query='select FreeSpace, DeviceID FROM Win32_LogicalDisk'"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Contains(t, string(res.BuildPluginOutput()), "C:", "output matches")

	StopTestAgent(t, snc)
}
