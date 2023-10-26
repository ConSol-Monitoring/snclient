package main

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	localINIPath = `C:\Program Files\snclient\snclient_local.ini`

	buildMSITimeout = 3 * time.Minute
)

var requiredFiles = []string{
	"snclient.exe",
	"snclient.ini",
	"LICENSE",
	"cacert.pem",
	"server.key",
	"server.crt",
	"README.md",
}

// this test requires the wix.exe to be installed
// it builds the msi file, tries a installation and removes it afterwards
func TestMSIinstaller(t *testing.T) {
	bin := getBinary()
	require.FileExistsf(t, bin, "snclient binary must exist")

	require.FileExistsf(t, "../snclient.msi", "snclient.msi binary must exist")

	// install msi file
	runCmd(t, &cmd{
		Dir:  "..",
		Cmd:  "msiexec",
		Args: []string{"/i", "snclient.msi", "/qn"},
	})

	for _, file := range requiredFiles {
		require.FileExistsf(t, `C:\Program Files\snclient\`+file, file+" has been installed")
	}

	// verify installation
	runCmd(t, &cmd{
		Cmd:     "net",
		Args:    []string{"start", "snclient"},
		ErrLike: []string{"The requested service has already been started"},
		Exit:    -1,
	})

	// add custom .ini
	writeFile(t, localINIPath, localINI)
	writeFile(t, `snclient.ini`, localINI)

	// restart with new config
	runCmd(t, &cmd{Cmd: "net", Args: []string{"stop", "snclient"}})
	runCmd(t, &cmd{Cmd: "net", Args: []string{"start", "snclient"}})

	// verify response
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "check_snclient_version"},
		Like: []string{`^SNClient\+ v`},
	})

	// build second msi file to test upgrade
	runCmd(t, &cmd{
		Dir: "..",
		Cmd: `powershell`,
		Args: []string{
			`.\packaging\windows\build_msi.ps1`,
			"-out", "snclient_update.msi",
			"-major", "0",
			"-minor", "1",
			"-rev", "101",
			"-sha", "deadbeef",
		},
		Like:    []string{"Windows Installer", "snclient.wxs"},
		Timeout: buildMSITimeout,
	})

	// install update from msi file
	runCmd(t, &cmd{
		Dir:  "..",
		Cmd:  `msiexec`,
		Args: []string{"/i", "snclient_update.msi", "/qn"},
	})

	for _, file := range requiredFiles {
		require.FileExistsf(t, `C:\Program Files\snclient\`+file, file+" still exists")
	}

	// verify response
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "check_snclient_version"},
		Like: []string{`^SNClient\+ v`},
	})
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "check_uptime", "crit=uptime<2s", "warn=uptime<1s"},
		Like: []string{"OK: uptime"},
	})

	// cleanup
	os.Remove(localINIPath)

	// uninstall msi file
	runCmd(t, &cmd{
		Dir:  "..",
		Cmd:  `msiexec`,
		Args: []string{"/x", "snclient_update.msi", "/qn"},
	})

	for _, file := range requiredFiles {
		assert.NoFileExistsf(t, `C:\Program Files\snclient\`+file, file+" has been removed")
	}
	assert.NoFileExistsf(t, `C:\Program Files\snclient\`, "snclient folder has been removed")

	// remove remaining files
	os.Remove("../snclient_update.msi")
	os.Remove("snclient.ini")
}
