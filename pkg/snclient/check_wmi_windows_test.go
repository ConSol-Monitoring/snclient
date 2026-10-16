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

func TestCheckWMINamespace(t *testing.T) {
	config := `
	[/modules]
	CheckWMI = enabled
	`
	snc := StartTestAgent(t, config)

	// AntiVirusProduct only exists in root\SecurityCenter2 (not the default
	// root\cimv2), so this only passes if the namespace is actually used.
	res := snc.RunCheck("check_wmi", []string{"query='select displayName FROM AntiVirusProduct'", "namespace='root\SecurityCenter2'"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")

	StopTestAgent(t, snc)
}

func TestCheckWMIPerfCounterAvailableMBytes(t *testing.T) {
	config := `
	[/modules]
	CheckWMI = enabled
	`
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_wmi", []string{"query='SELECT Name,AvailableMBytes FROM Win32_PerfRawData_PerfOS_Memory'"})
	res.ParsePerformanceDataFromOutput()
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Contains(t, string(res.BuildPluginOutput()), "|'AvailableMBytes'=", "AvailableMBytes available in perfdata")

	StopTestAgent(t, snc)
}

func TestCheckWMIPerfCounterMultiple(t *testing.T) {
	config := `
	[/modules]
	CheckWMI = enabled
	`
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_wmi", []string{"query='select DeviceID, FreeSpace, Size FROM Win32_LogicalDisk'"})
	res.ParsePerformanceDataFromOutput()
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Contains(t, string(res.BuildPluginOutput()), "'C: FreeSpace'=", "C: FreeSpace found in perfdata")
	assert.Contains(t, string(res.BuildPluginOutput()), "'C: Size'=", "Size found in perfdata")

	StopTestAgent(t, snc)
}
