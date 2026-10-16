package snclient

import (
	"strings"
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

	// cimv2 is a child of root, not of the default root\cimv2 namespace.
	res := snc.RunCheck("check_wmi", []string{
		`query=SELECT Name FROM __NAMESPACE WHERE Name = 'cimv2'`,
		`namespace=root`,
	})
	output := string(res.BuildPluginOutput())
	assert.Equalf(t, CheckExitOK, res.State, "unexpected result: %s", output)
	assert.Contains(t, strings.ToLower(output), "cimv2")

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
