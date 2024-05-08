package main

import (
	"os"
	"runtime"
	"testing"

	snclientutils "github.com/consol-monitoring/snclient/pkg/utils"
	"github.com/stretchr/testify/require"
)

var localINI = `
[/modules]
CheckBuiltinPlugins = enabled
CheckExternalScripts = enabled

[/settings/default]
password = test

[/settings/updates/channel]
local = file://./tmpupdates/snclient${file-ext}
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

func TestCommandUpdate(t *testing.T) {
	bin := getBinary()
	require.FileExistsf(t, bin, "snclient binary must exist")

	writeFile(t, `snclient.ini`, localINI)

	err := os.Mkdir("tmpupdates", 0o700)
	require.NoError(t, err)

	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	err = snclientutils.CopyFile(bin, "./tmpupdates/snclient"+suffix)
	require.NoError(t, err)

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"update", "local", "--force"},
		Like: []string{`successfully downloaded`, `applied successfully`},
	})

	defer os.RemoveAll("./tmpupdates")

	os.Remove("snclient.ini")
}
