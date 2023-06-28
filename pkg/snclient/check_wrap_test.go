package snclient

import (
	"regexp"
	"testing"
	"fmt"

	"github.com/stretchr/testify/assert"
)

func init() {
	fmt.Println("Initializing external command tests...")
	//log.SetLevel(log.DebugLevel)
}

func TestCheckWrap(t *testing.T) {
	config := `
[/modules]
CheckExternalScripts = enabled

[/settings/external scripts/scripts]
check_dummy_sh = scripts/check_dummy.sh
check_dummy_sh_ok = scripts/check_dummy.sh OK "i am ok"
`
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
