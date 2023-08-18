//go:build !windows

package snclient

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckWrapUnix(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := fmt.Sprintf(`
[/modules]
CheckExternalScripts = enabled

[/settings/external scripts/scripts]
check_dummy_sh = %s/check_dummy.sh
check_dummy_sh_ok = %s/check_dummy.sh 0 "i am ok"
check_dummy_sh_critical = %s/check_dummy.sh 2 "i am critical"

[/settings/external scripts/scripts/timeoutscript]
timeout = 1
command = sleep 10
`, scriptsDir, scriptsDir, scriptsDir)

	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_dummy_sh_ok", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Equalf(t, "OK: i am ok", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_sh_critical", []string{})
	assert.Equalf(t, CheckExitCritical, res.State, "state OK")
	assert.Equalf(t, "CRITICAL: i am critical", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("timeoutscript", []string{})
	assert.Equalf(t, CheckExitUnknown, res.State, "state Unknown")
	assert.Equalf(t, "UNKNOWN: script run into timeout after 1s\n", string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}
