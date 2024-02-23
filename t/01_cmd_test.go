package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

var localINI = `
[/modules]
CheckBuiltinPlugins = enabled
CheckExternalScripts = enabled

[/settings/default]
password = test

[/settings/external scripts/scripts]
check_win_not_exist1 = C:\Program Files\test\test.exe
check_win_not_exist2 = C:\Program Files\te st\test.exe
check_win_snclient_version = C:\Program Files\snclient\snclient.exe -V
check_win_snclient_version1 = C:\Program Files\snclient\snclient.exe -V
check_win_snclient_version2 = 'C:\Program Files\snclient\snclient.exe' -V
check_win_snclient_version3 = "C:\Program Files\snclient\snclient.exe" -V
check_win_snclient_version4 = & 'C:\Program Files\snclient\snclient.exe' -V
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

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"hash", "test"},
		Like: []string{`hash sum: SHA256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08`},
	})

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"inventory"},
		Like: []string{`uptime`, `inventory`},
	})

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"inventory", "uptime"},
		Like: []string{`uptime`, `inventory`},
	})

	os.Remove("snclient.ini")
}
