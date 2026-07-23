package main

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	localDEBINIPath = `/etc/snclient/snclient_local.ini`
	aptGetTimeout   = 3 * time.Minute
)

// this test requires:
// - snclient.deb
// it tries a installation and removes it afterwards
func TestDEBinstaller(t *testing.T) {
	// skip test unless requested, it will uninstall existing installations
	if os.Getenv("SNCLIENT_INSTALL_TEST") != "1" {
		t.Skipf("SKIPPED: pkg installer test requires env SNCLIENT_INSTALL_TEST=1")

		return
	}

	bin := "/usr/bin/snclient"
	require.FileExistsf(t, "snclient.deb", "snclient.deb binary must exist")

	// install deb file
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"apt-get", "install", "-y", "./snclient.deb"},
		Env: map[string]string{
			"DEBIAN_FRONTEND":     "noninteractive",
			"NEEDRESTART_SUSPEND": "1",
		},
		ErrLike: []string{`.*`},
		Timeout: aptGetTimeout,
	})

	requiredFiles := []string{
		"/usr/bin/snclient",
		"/usr/lib/snclient/node_exporter",
		"/etc/snclient/snclient.ini",
		"/etc/snclient/cacert.pem",
		"/etc/snclient/server.key",
		"/etc/snclient/server.crt",
		"/usr/lib/systemd/system/snclient.service",
		"/etc/logrotate.d/snclient",
	}
	for _, file := range requiredFiles {
		require.FileExistsf(t, file, file+" has been installed")
	}
	requiredFolders := []string{
		"/var/lib/snclient",
		"/var/log/snclient",
	}
	for _, folder := range requiredFolders {
		require.DirExistsf(t, folder, folder+" has been created")
	}

	runCmd(t, &cmd{
		Cmd:  "/usr/bin/snclient",
		Args: []string{"-V"},
		Like: []string{`^SNClient v`},
	})

	runCmd(t, &cmd{
		Cmd:  "systemctl",
		Args: []string{"is-active", "--quiet", "snclient"},
	})

	// Check the configured command separately. The process tree in the
	// human-readable status output may temporarily contain only the process
	// name while the service is starting.
	runCmd(t, &cmd{
		Cmd:  "systemctl",
		Args: []string{"show", "--property=ExecStart", "--value", "snclient"},
		Like: []string{`/usr/bin/snclient`},
	})

	// add custom .ini with correct ownership for snclient user
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"touch", localDEBINIPath},
	})
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"chmod", "666", localDEBINIPath},
	})
	writeFile(t, localDEBINIPath, localTestINI)
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"chown", "snclient:snclient", localDEBINIPath},
	})
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"chmod", "640", localDEBINIPath},
	})
	writeFile(t, `snclient.ini`, localDaemonINI)

	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"systemctl", "restart", "snclient"},
	})

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

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "check_cpu", "crit=load>101", "warn=load>101"},
		Like: []string{"OK - CPU load is ok."},
	})

	// do some extra check when in defined test environment
	if os.Getenv("SNCLIENT_TEST_SETUP") == "container" {
		localContainerTests(t, bin)
	}

	// make logfolder and logfile readable and check for errors
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"chmod", "755", "/var/log/snclient"},
	})
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"chmod", "666", "/var/log/snclient/snclient.log"},
	})
	logContent, err := os.ReadFile("/var/log/snclient/snclient.log")
	require.NoError(t, err)
	assert.NotContainsf(t, string(logContent), "[Error]", "log does not contain errors")

	// cleanup
	os.Remove(localDEBINIPath)

	// uninstall pkg file
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"apt-get", "remove", "-y", "--purge", "snclient"},
		Env: map[string]string{
			"DEBIAN_FRONTEND":     "noninteractive",
			"NEEDRESTART_SUSPEND": "1",
		},
		Timeout: aptGetTimeout,
	})

	for _, file := range requiredFiles {
		assert.NoFileExistsf(t, file, file+" has been removed")
	}

	// remove remaining files
	os.Remove("snclient.ini")
}

func localContainerTests(t *testing.T, bin string) {
	t.Helper()

	// extend configuration
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"systemctl", "restart", "dbus"},
	})
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"cp", "/var/tmp/snclient_local_docker.ini", "/etc/snclient/"},
	})
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"cp", "/usr/bin/snclient", "/var/www/html/snclient.linux"},
	})
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"systemctl", "restart", "apache2"},
	})
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"systemctl", "restart", "snclient"},
	})
	waitUntilResponse(t, bin)

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "sudo_id"},
		Like: []string{"uid=0\\(root\\)"},
	})
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "nosudo_id"},
		Like: []string{"uid=\\d*\\(snclient\\)"},
	})
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "capsh"},
		Like: []string{"Current: ="},
	})
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "check_service", "service=snclient"},
		Like: []string{"OK - All 1 service\\(s\\) are ok.*snclient"},
	})
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "check_service", "service='snc\ntest'"},
		Like: []string{"request contained illegal control characters"},
		Exit: 3,
	})
	// check if check_omd still works
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "check_omd"},
		Like: []string{"UNKNOWN - failed to fetch omd sites"},
		Exit: 3,
	})
	// but not like this
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "check_omd", "site='te\rst'"},
		Like: []string{"request contained illegal control characters"},
		Exit: 3,
	})

	// test admin api
	runCmd(t, &cmd{
		Cmd:  "curl",
		Args: []string{"-sS", "-k", "--header", "password:admin", "-d", "''", "https://localhost:8443/api/v1/admin/reload"},
		Like: []string{`{"success":true}`},
		Exit: 0,
	})
	waitUntilResponse(t, bin)

	// increase log level
	runCmd(t, &cmd{
		Cmd:  "curl",
		Args: []string{"-sS", "-k", "--header", "password:admin", "-d", `{"level":"debug", "duration": 600}`, "https://localhost:8443/api/v1/admin/log/level"},
		Like: []string{`{.*"success":true.*}`},
		Exit: 0,
	})

	// test admin api local update
	runCmd(t, &cmd{
		Cmd:  "curl",
		Args: []string{"-sS", "-k", "--header", "password:admin", "-d", `{"channel":"local_file", "force": true}`, "https://localhost:8443/api/v1/admin/updates/install"},
		Like: []string{`update found and installed`},
	})
	time.Sleep(3 * time.Second)
	waitUntilResponse(t, bin)

	runCmd(t, &cmd{
		Cmd:  "bash",
		Args: []string{"-c", `ls -la /proc/$(pidof snclient)/exe`},
		Like: []string{`/var/cache/snclient/snclient`},
		Exit: 0,
	})
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"systemctl", "restart", "snclient"},
	})
	waitUntilResponse(t, bin)
	runCmd(t, &cmd{
		Cmd:  "bash",
		Args: []string{"-c", `ls -la /proc/$(pidof snclient)/exe`},
		Like: []string{`/var/cache/snclient/snclient`},
		Exit: 0,
	})

	// test admin api http update
	runCmd(t, &cmd{
		Cmd:  "curl",
		Args: []string{"-sS", "-k", "--header", "password:admin", "-d", `{"channel":"local_file", "force": true}`, "https://localhost:8443/api/v1/admin/updates/install"},
		Like: []string{`update found and installed`},
	})
	time.Sleep(3 * time.Second)
	waitUntilResponse(t, bin)

	runCmd(t, &cmd{
		Cmd:  "bash",
		Args: []string{"-c", `ls -la /proc/$(pidof snclient)/exe`},
		Like: []string{`/var/cache/snclient/snclient`},
		Exit: 0,
	})
	runCmd(t, &cmd{
		Cmd:  "sudo",
		Args: []string{"systemctl", "restart", "snclient"},
	})
	waitUntilResponse(t, bin)
	runCmd(t, &cmd{
		Cmd:  "bash",
		Args: []string{"-c", `ls -la /proc/$(pidof snclient)/exe`},
		Like: []string{`/var/cache/snclient/snclient`},
		Exit: 0,
	})
}
