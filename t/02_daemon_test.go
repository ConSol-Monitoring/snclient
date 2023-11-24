package main

import (
	"fmt"
	"os"
	"pkg/utils"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	localDaemonPort     = 40555
	localDaemonPassword = "test"
	localDaemonINI      = `
[/modules]
CheckBuiltinPlugins = enabled

[/settings/default]
password = ` + localDaemonPassword + `

[/settings/WEB/server]
use ssl = disabled
port = ` + fmt.Sprintf("%d", localDaemonPort) + `
`
)

func TestDaemonRequests(t *testing.T) {
	bin := getBinary()
	require.FileExistsf(t, bin, "snclient binary must exist")

	writeFile(t, `snclient.ini`, localDaemonINI)
	pidFile := "snclient.lock"
	pid := 0

	// start daemon
	go func() {
		res := runCmd(t, &cmd{
			Cmd:    bin,
			Args:   []string{"-vv", "-logfile", "stdout", "-pidfile", pidFile},
			Like:   []string{"starting", "listener on", "got sigterm", "snclient exited"},
			Unlike: []string{"PANIC"},
		})
		t.Logf("daemon finished")
		assert.Emptyf(t, res.Stderr, "stderr should be empty")
	}()

	startTimeOut := time.Now().Add(10 * time.Second)
	for {
		if time.Now().After(startTimeOut) {
			break
		}
		pidN, err := utils.ReadPid(pidFile)
		if err == nil {
			pid = pidN

			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	require.Greaterf(t, pid, 0, "daemon started")

	t.Logf("daemon started with pid: %d", pid)
	time.Sleep(500 * time.Millisecond)

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-p", localDaemonPassword, "-u", fmt.Sprintf("http://127.0.0.1:%d", localDaemonPort)},
		Like: []string{`OK: REST API reachable on http:`},
	})

	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_nsc_web", "-p", localDaemonPassword, "-r", "-u", fmt.Sprintf("http://127.0.0.1:%d/api/v1/inventory", localDaemonPort)},
		Like: []string{`{"inventory":`},
	})

	t.Logf("test done, shuting down")
	process, err := os.FindProcess(pid)
	require.NoErrorf(t, err, "find daemon process")

	err = process.Signal(syscall.SIGTERM)
	require.NoErrorf(t, err, "killing daemon")
	os.Remove(pidFile)
	os.Remove("snclient.ini")
	time.Sleep(500 * time.Millisecond)
}
