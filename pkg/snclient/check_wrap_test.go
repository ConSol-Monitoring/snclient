package snclient

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckWrap(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := fmt.Sprintf(`
[/modules]
CheckExternalScripts = enabled

[/settings/external scripts/scripts]
check_dummy_sh = %s/check_dummy.sh
check_dummy_sh_ok = %s/check_dummy.sh OK "i am ok"
`, scriptsDir, scriptsDir)

	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_dummy_sh_ok", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Equalf(t, "OK: i am ok", string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}
