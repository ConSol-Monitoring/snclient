package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	localOSXINIPath = `/etc/snclient/snclient_local.ini`
)

// this test requires:
// - snclient.pkg
// it tries a installation and removes it afterwards
func TestOSXinstaller(t *testing.T) {
	// skip test unless requested, it will uninstall existing installations
	if os.Getenv("SNCLIENT_INSTALL_TEST") != "1" {
		t.Skipf("SKIPPED: pkg installer test requires env SNCLIENT_INSTALL_TEST=1")

		return
	}

	bin := getBinary()
	require.FileExistsf(t, "snclient.pkg", "snclient.pkg binary must exist")

	// install pkg file
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"/usr/sbin/installer", "-pkg", "snclient.pkg", "-target", "/"},
	})

	requiredFiles := []string{
		"/usr/local/bin/snclient",
		"/usr/local/bin/snclient_uninstall.sh",
		"/usr/local/bin/node_exporter",
		"/etc/snclient/snclient.ini",
		"/etc/snclient/cacert.pem",
		"/etc/snclient/server.key",
		"/etc/snclient/server.crt",
		"/Library/LaunchDaemons/com.snclient.snclient.plist",
	}
	for _, file := range requiredFiles {
		require.FileExistsf(t, file, file+" has been installed")
	}

	// add custom .ini
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"touch", localOSXINIPath},
	})
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"chmod", "666", localOSXINIPath},
	})
	writeFile(t, localOSXINIPath, localTestINI)
	writeFile(t, `snclient.ini`, localINI)

	// restart
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"launchctl", "unload", "/Library/LaunchDaemons/com.snclient.snclient.plist"},
	})
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"launchctl", "load", "/Library/LaunchDaemons/com.snclient.snclient.plist"},
	})

	waitUntilResponse(t, bin)

	// verify response
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "check_snclient_version"},
		Like: []string{`^SNClient\+ v`},
	})

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "check_uptime", "crit=uptime<2s", "warn=uptime<1s"},
		Like: []string{"OK - uptime"},
	})

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "check_cpu", "crit=load>101", "warn=load>101"},
		Like: []string{"OK - CPU load is ok."},
	})

	// cleanup
	os.Remove(localOSXINIPath)

	// uninstall pkg file
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"/usr/local/bin/snclient_uninstall.sh"},
	})

	for _, file := range requiredFiles {
		assert.NoFileExistsf(t, file, file+" has been removed")
	}

	// remove remaining files
	os.Remove("snclient.ini")
}
