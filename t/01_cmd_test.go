package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var localINI = `
[/modules]
CheckBuiltinPlugins = enabled

[/settings/default]
password = test
`

func TestCommandFlags(t *testing.T) {
	bin := getBinary()
	require.FileExistsf(t, bin, "snclient binary must exist")

	writeFile(t, `snclient.ini`, localINI)

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"-V"},
		Like: []string{"^SNClient.*Build:"},
	})

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_dummy", "help"},
		Like: []string{"check_dummy", "Usage"},
		Exit: 3,
	})

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_snclient_version"},
		Like: []string{`SNClient\+ v`},
	})
}
