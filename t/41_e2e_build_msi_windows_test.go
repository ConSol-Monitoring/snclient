package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	localINIPath = `C:\Program Files\snclient\snclient_local.ini`

	installMSITimeout = 1 * time.Minute
	buildMSITimeout   = 3 * time.Minute
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

// this test requires the wix.exe (including .net 3.5) to be installed
// further requirements are:
// - snclient.msi
// - windist folder to build new msi (incl. snclient.exe and windows_exporter.exe)
// it builds the msi file, tries a installation and removes it afterwards
func TestMSIinstaller(t *testing.T) {
	// skip test unless requested, it will uninstall existing installations
	if os.Getenv("SNCLIENT_INSTALL_TEST") != "1" {
		t.Skipf("SKIPPED: pkg installer test requires env SNCLIENT_INSTALL_TEST=1")

		return
	}

	bin := getBinary()
	require.FileExistsf(t, bin, "snclient binary must exist")
	require.FileExistsf(t, "snclient.msi", "snclient.msi binary must exist")
	require.FileExistsf(t, "snclient.exe", "snclient.exe binary must exist")
	require.DirExistsf(t, "../windist", "windist folder must exist")
	require.FileExistsf(t, "../windist/snclient.exe", "windist/snclient.exe must exist")
	require.FileExistsf(t, "../windist/windows_exporter.exe", "windist/windows_exporter.exe must exist")

	// install msi file
	runCmd(t, &cmd{
		Cmd:     "msiexec",
		Args:    []string{"/i", "snclient.msi", "/qn", "ALLOWEDHOSTS=127.0.0.1,::1,192.168.0.1"},
		Timeout: installMSITimeout,
	})

	for _, file := range requiredFiles {
		require.FileExistsf(t, `C:\Program Files\snclient\`+file, file+" has been installed")
	}

	// Verify the packaged binary recognizes a volume mounted at a directory.
	volumeMountPath := setupDirectoryMountedVolume(t)
	runCmd(t, &cmd{
		Cmd: `C:\Program Files\snclient\snclient.exe`,
		Args: []string{
			"test",
			"check_drivesize",
			"warn=used>=95%",
			"crit=used>=97%",
			"drive=" + volumeMountPath,
			"perf-config=*(unit:Gb)",
			"perf-syntax=%(key:lc)",
			"show-all",
		},
		Like: []string{
			`Exit Code: OK \(0\)`,
			`OK.*disk3`,
		},
	})

	iniContent, err := os.ReadFile(`C:\Program Files\snclient\snclient.ini`)
	require.NoErrorf(t, err, "snclient.ini can be read")
	assert.Containsf(t, string(iniContent), "allowed hosts = 127.0.0.1,::1,192.168.0.1", "snclient.ini contains allowed hosts")

	// make sure snclient.exe has a file version set
	runCmd(t, &cmd{
		Cmd:    bin,
		Args:   []string{"run", "check_files", `path='C:\Program Files\snclient\snclient.exe'`, "detail-syntax='%{name} %{version}'", "top-syntax='%(problem_list)'", "crit=version = 0", "show-all"},
		Like:   []string{`OK - snclient.exe \d+`},
		Unlike: []string{`%\{version\}`, `0.0.0.0`},
	})

	// verify installation
	runCmd(t, &cmd{
		Cmd:     "net",
		Args:    []string{"start", "snclient"},
		ErrLike: []string{"The requested service has already been started"},
		Exit:    -1,
	})

	// add custom .ini
	writeFile(t, localINIPath, localTestINI)
	writeFile(t, `snclient.ini`, localDaemonINI)

	// restart with new config
	runCmd(t, &cmd{Cmd: "net", Args: []string{"stop", "snclient"}})
	runCmd(t, &cmd{Cmd: "net", Args: []string{"start", "snclient"}})

	// wait a couple of seconds till daemon answers
	waitUntilResponse(t, bin)

	// verify response
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "check_snclient_version"},
		Like: []string{`^SNClient v`},
	})

	// build second msi file (from the parent folder) to test upgrade
	runCmd(t, &cmd{
		Dir: "..",
		Cmd: `powershell`,
		Args: []string{
			`.\packaging\windows\build_msi.ps1`,
			"-out", `.\t\snclient_update.msi`,
			"-major", "99",
			"-minor", "1",
			"-rev", "101",
			"-sha", "deadbeef",
		},
		Like:    []string{"Windows Installer", "snclient.wxs"},
		Timeout: buildMSITimeout,
	})

	// install update from msi file
	runCmd(t, &cmd{
		Cmd:     `msiexec`,
		Args:    []string{"/i", "snclient_update.msi", "/qn"},
		Timeout: installMSITimeout,
	})

	for _, file := range requiredFiles {
		require.FileExistsf(t, `C:\Program Files\snclient\`+file, file+" still exists")
	}

	waitUntilResponse(t, bin)

	// verify response
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "check_snclient_version"},
		Like: []string{`^SNClient v`},
	})
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "check_uptime", "crit=uptime<2s", "warn=uptime<1s"},
		Like: []string{"OK - uptime"},
	})

	// run check with known path which contains spaces
	for _, num := range []string{"1", "2", "3", "4", "5"} {
		runCmd(t, &cmd{
			Cmd:  bin,
			Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "check_win_snclient_test" + num},
			Like: []string{`testpattern`},
			Exit: 3,
		})
	}

	// run check with known not-existing path which contains spaces
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "check_win_not_exist1"},
		Like: []string{`UNKNOWN - Return code of 127 is out of bounds.`},
		Exit: 3,
	})

	// check a stopped daemon
	runCmd(t, &cmd{Cmd: "net", Args: []string{"stop", "Spooler"}})
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-a", "legacy", "-u", "https://localhost:8443", "check_service", "service=Spooler", "warn=state!=started", "crit=none"},
		Like: []string{"Spooler=stopped", "'Spooler rss'=U"},
		Exit: 1,
	})
	runCmd(t, &cmd{
		Cmd: bin,
		Args: []string{
			"run", "check_nsc_web", "-k", "-p", "test", "-a", "1", "-u", "https://localhost:8443",
			"check_service", "service=Spooler", "warn=state!=started", "crit=none",
		},
		Like: []string{"Spooler=stopped", "'Spooler rss'=U"},
		Exit: 1,
	})
	runCmd(t, &cmd{
		Cmd: bin,
		Args: []string{
			"run", "check_nsc_web", "-k", "-p", "test", "-a", "1", "-u", "https://localhost:8443",
			"check_service", "service=Spooler", "warn=state!=started", "crit=none", "perf-config=*(magic:2)",
		},
		Like: []string{"Spooler=stopped", "'Spooler rss'=U"},
		Exit: 1,
	})
	runCmd(t, &cmd{Cmd: "net", Args: []string{"start", "Spooler"}})

	logContent, err := os.ReadFile("c:\\Program Files\\snclient\\snclient.log")
	require.NoError(t, err)
	assert.NotContainsf(t, string(logContent), "[Error]", "log does not contain errors")

	// cleanup
	os.Remove(localINIPath)

	// uninstall msi file
	runCmd(t, &cmd{
		Cmd:     `msiexec`,
		Args:    []string{"/x", "snclient_update.msi", "/qn"},
		Timeout: installMSITimeout,
	})

	for _, file := range requiredFiles {
		assert.NoFileExistsf(t, `C:\Program Files\snclient\`+file, file+" has been removed")
	}
	assert.NoFileExistsf(t, `C:\Program Files\snclient\`, "snclient folder has been removed")

	// remove remaining files
	os.Remove("snclient_update.msi")
	os.Remove("snclient.ini")
}

func setupDirectoryMountedVolume(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	vhdDir := filepath.Join(root, "vhds")
	mountPath := filepath.Join(root, "testmount", "disk3")
	require.NoError(t, os.MkdirAll(vhdDir, 0o700), "creating VHD directory")
	require.NoError(t, os.MkdirAll(mountPath, 0o700), "creating volume mount directory")

	vhdPath := filepath.Join(vhdDir, "snclient-drivesize-test.vhdx")
	scriptDir := t.TempDir()
	createScript := filepath.Join(scriptDir, "create-volume.txt")
	writeFile(t, createScript, fmt.Sprintf(`create vdisk file="%s" maximum=100 type=expandable
select vdisk file="%s"
attach vdisk
convert gpt
create partition primary
format fs=ntfs quick label="SNClientTest"
assign mount="%s"
`, vhdPath, vhdPath, mountPath))

	runCmd(t, &cmd{Cmd: "diskpart", Args: []string{"/s", createScript}})
	t.Cleanup(func() {
		detachScript := filepath.Join(scriptDir, "detach-volume.txt")
		writeFile(t, detachScript, fmt.Sprintf(`select vdisk file="%s"
detach vdisk
`, vhdPath))
		runCmd(t, &cmd{Cmd: "diskpart", Args: []string{"/s", detachScript}})
	})

	return mountPath
}
