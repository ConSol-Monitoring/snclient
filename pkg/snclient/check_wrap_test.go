package snclient

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func init() {
	fmt.Println("Initializing external command tests...")
	// log.SetLevel(log.DebugLevel)
}

func TestCheckWrap(t *testing.T) {
	_, myTestFile, _, _ := runtime.Caller(0)
	myTestDir := filepath.Dir(myTestFile)
	scriptsDir := filepath.Join(myTestDir, "t", "scripts")

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
	assert.Regexpf(t,
		regexp.MustCompile(`^OK: i am ok$`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	StopTestAgent(t, snc)
}
