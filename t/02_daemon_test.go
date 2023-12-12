package main

import (
	"fmt"
	"os"
	"testing"

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
`
)

func TestDaemonRequests(t *testing.T) {
	bin := getBinary()
	require.FileExistsf(t, bin, "snclient binary must exist")

	writeFile(t, `snclient.ini`, localDaemonINI)

	startBackgroundDaemon(t)

	baseArgs := []string{"run", "check_nsc_web", "-p", localDaemonPassword, "-u", fmt.Sprintf("http://127.0.0.1:%d", localDaemonPort)}

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: baseArgs,
		Like: []string{`OK: REST API reachable on http:`},
	})

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-p", localDaemonPassword, "-r", "-u", fmt.Sprintf("http://127.0.0.1:%d/api/v1/inventory", localDaemonPort)},
		Like: []string{`{"inventory":`},
	})

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "check_cpu", "warn=load > 100", "crit=load > 100"),
		Like: []string{`OK: CPU load is ok.`},
	})

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "check_network", "warn=total > 100000000", "crit=total > 100000000"),
		Like: []string{`OK: \w+ >\d+ \w*B/s <\d+ \w*B\/s`},
	})

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-p", localDaemonPassword, "-r", "-u", fmt.Sprintf("http://127.0.0.1:%d/api/v1/admin/reload", localDaemonPort)},
		Like: []string{`RESPONSE-ERROR: http request failed: 403 Forbidden`},
		Exit: 3,
	})

	stopBackgroundDaemon(t)
	os.Remove("snclient.ini")
}
