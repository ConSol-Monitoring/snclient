package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	localDaemonPort          = 40555
	localDaemonPassword      = "test"
	localDaemonAdminPassword = "admin"
	localDaemonINI           = `
[/modules]
CheckBuiltinPlugins = enabled
WEBAdminServer = enabled

[/settings/default]
password = ` + localDaemonPassword + `
certificate = test.crt
certificate key = test.key

[/settings/WEB/server]
use ssl = disabled
port = ` + fmt.Sprintf("%d", localDaemonPort) + `

[/settings/WEBAdmin/server]
use ssl = disabled
port = ` + fmt.Sprintf("%d", localDaemonPort) + `
password = ` + localDaemonAdminPassword + `

[/settings/external scripts/scripts]
check_echo = echo '%ARG1%'
check_echo_win = cmd /c echo '%ARG1%'
check_win_snclient_version = C:\Program Files\snclient\snclient.exe -V
check_win_snclient_version1 = C:\Program Files\snclient\snclient.exe -V
check_win_snclient_version2 = 'C:\Program Files\snclient\snclient.exe' -V
check_win_snclient_version3 = "C:\Program Files\snclient\snclient.exe" -V
check_win_snclient_version4 = & 'C:\Program Files\snclient\snclient.exe' -V
`
)

func TestDaemonRequests(t *testing.T) {
	bin := getBinary()
	require.FileExistsf(t, bin, "snclient binary must exist")

	writeFile(t, `snclient.ini`, localDaemonINI)

	pid := startBackgroundDaemon(t)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", localDaemonPort)

	t.Logf("daemon started: %d", pid)

	baseArgs := []string{"run", "check_nsc_web", "-p", localDaemonPassword, "-u", baseURL}

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: baseArgs,
		Like: []string{`OK - REST API reachable on http:`},
	})

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-p", localDaemonPassword, "-r", "-u", fmt.Sprintf("%s/api/v1/inventory", baseURL)},
		Like: []string{`{"inventory":`},
	})

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "check_cpu", "warn=load > 100", "crit=load > 100"),
		Like: []string{`OK - CPU load is ok.`},
	})

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "check_network", "warn=total > 100000000", "crit=total > 100000000"),
		Like: []string{`OK - \w+ >\d+ \w*B/s <\d+ \w*B\/s`},
	})

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-p", localDaemonPassword, "-r", "-u", baseURL + "/api/v1/admin/reload"},
		Like: []string{`RESPONSE-ERROR: http request failed: 403 Forbidden`},
		Exit: 3,
	})

	runCmd(t, &cmd{
		Cmd:  "curl",
		Args: []string{"-s", "-u", "user:" + localDaemonAdminPassword, "-k", baseURL + "/api/v1/admin/reload"},
		Like: []string{`POST method required`},
	})

	runCmd(t, &cmd{
		Cmd:  "curl",
		Args: []string{"-s", "-u", "user:" + localDaemonAdminPassword, "-k", "-X", "POST", baseURL + "/api/v1/admin/reload"},
		Like: []string{`{"success":true}`},
	})

	postData, err := json.Marshal(map[string]string{
		"Unknown": "false",
	})
	require.NoErrorf(t, err, "post data json encoded")
	runCmd(t, &cmd{
		Cmd:  "curl",
		Args: []string{"-s", "-u", "user:" + localDaemonAdminPassword, "-k", "-s", "-d", string(postData), baseURL + "/api/v1/admin/certs/replace"},
		Like: []string{`unknown field`},
	})

	// test replacing certificates
	err = os.WriteFile("test.crt", []byte{}, 0o600)
	require.NoErrorf(t, err, "test.crt truncated")
	err = os.WriteFile("test.key", []byte{}, 0o600)
	require.NoErrorf(t, err, "test.key truncated")
	postData, err = json.Marshal(map[string]interface{}{
		"Reload":   true,
		"CertData": "dGVzdGNlcnQ=",
		"KeyData":  "dGVzdGtleQ==",
	})
	require.NoErrorf(t, err, "post data json encoded")
	runCmd(t, &cmd{
		Cmd:  "curl",
		Args: []string{"-s", "-u", "user:" + localDaemonAdminPassword, "-k", "-s", "-d", string(postData), baseURL + "/api/v1/admin/certs/replace"},
		Like: []string{`{"success":true}`},
	})
	crt, _ := os.ReadFile("test.crt")
	key, _ := os.ReadFile("test.key")

	assert.Equalf(t, "testcert", string(crt), "test certificate written")
	assert.Equalf(t, "testkey", string(key), "test certificate key written")

	ok := stopBackgroundDaemon(t)
	assert.Truef(t, ok, "stopping worked")
	os.Remove("snclient.ini")
}
