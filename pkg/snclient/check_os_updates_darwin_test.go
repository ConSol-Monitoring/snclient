package snclient

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckOSXUpdates(t *testing.T) {
	snc := StartTestAgent(t, "")

	// mock softwareupdate command from output of: softwareupdate -l --no-scan
	tmpPath := MockSystemUtilities(t, map[string]string{
		"softwareupdate": `
Software Update found the following new or updated software:
* Label: macOS Sonoma 14.3.1-23D60
Title: macOS Sonoma 14.3.1, Version: 14.3.1, Size: 1789479KiB, Recommended: NO, Action: restart,`,
	})
	defer os.RemoveAll(tmpPath)
	res := snc.RunCheck("check_os_updates", []string{"--system=osx"})
	assert.Equalf(t, CheckExitWarning, res.State, "state WARNING")
	assert.Containsf(t, string(res.BuildPluginOutput()), "WARNING - 0 security updates / 1 updates available. |'security'=0;;0;0 'updates'=1;0;;0", "output matches")

	StopTestAgent(t, snc)
}
