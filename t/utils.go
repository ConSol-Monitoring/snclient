package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/consol-monitoring/snclient/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	DefaultCommandTimeout = 30 * time.Second
)

var localTestINI = `
[/modules]
CheckBuiltinPlugins = enabled
CheckExternalScripts = enabled

[/settings/default]
password = test

[/settings/external scripts/scripts]
check_win_not_exist1 = C:\Program Files\test\test.exe
check_win_not_exist2 = C:\Program Files\te st\test.exe
check_win_snclient_test1 =    C:\Program Files\snclient\snclient.exe  run check_dummy 3 testpattern
check_win_snclient_test2 =   'C:\Program Files\snclient\snclient.exe' run check_dummy 3 testpattern
check_win_snclient_test3 =   "C:\Program Files\snclient\snclient.exe" run check_dummy 3 testpattern
check_win_snclient_test4 = & 'C:\Program Files\snclient\snclient.exe' run check_dummy 3 testpattern
check_win_snclient_test5 = & Write-Host "testpattern"; exit 3
`

// cmdResult contains the result from the command
type cmdResult struct {
	Stdout   string
	Stderr   string
	ExitCode int64
}

// cmd contains the command along with assertions to run on the output
type cmd struct {
	Cmd  string   // the command to run (required)
	Args []string // arguments for the command
	Dir  string   // override work dir

	// assertions
	Like    []string // stdout must contain these lines (regexp)
	ErrLike []string // stderr must contain these lines (regexp), if nil, stderr must be empty
	Exit    int64    // exit code must match this number, set to -1 to accept all exit code (default 0)
	Unlike  []string // stdout must not contain these lines (regexp)

	// optional values when running a cmd
	Timeout time.Duration     // maximum run duration (default 30sec)
	Env     map[string]string // environment values

	CmdChannel chan *exec.Cmd // if set, cmd will be pushed there
}

// runCmd runs a test command defined from the Cmd struct
func runCmd(t *testing.T, opt *cmd) *cmdResult {
	t.Helper()
	assert.NotEmptyf(t, opt.Cmd, "command must not be empty")

	if opt.Timeout <= 0 {
		opt.Timeout = DefaultCommandTimeout
	}
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(opt.Timeout))
	defer cancel()

	check, outbuf, errbuf := prepareCmd(ctx, opt)

	if opt.CmdChannel != nil {
		opt.CmdChannel <- check
	}

	t.Logf("run: %s", check.String())
	err := check.Start()
	require.NoErrorf(t, err, "command started: %s", opt.Cmd)

	// https://github.com/golang/go/issues/18874
	// timeout does not work for child processes and/or if file handles are still open
	go procWatch(ctx, check.Process)

	err = check.Wait()
	cancel()

	// extract stdout and stderr
	res := &cmdResult{
		ExitCode: -1,
		Stdout:   outbuf.String(),
		Stderr:   errbuf.String(),
	}

	if err != nil && check.ProcessState == nil {
		logCmd(t, check, res)
		require.NoErrorf(t, err, "command wait: %s", opt.Cmd)

		return res
	}

	state := check.ProcessState

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		res.ExitCode = -1
		logCmd(t, check, res)
		assert.Fail(t, fmt.Sprintf("command run into timeout after %s", opt.Timeout.String()))

		return res
	}

	if waitStatus, ok := state.Sys().(syscall.WaitStatus); ok {
		res.ExitCode = int64(waitStatus.ExitStatus())
	}

	if opt.Exit != -1 {
		if opt.Exit != res.ExitCode {
			logCmd(t, check, res)
		}
		assert.Equalf(t, opt.Exit, res.ExitCode, "exit code is: %d", opt.Exit)
	}

	for _, l := range opt.Like {
		assert.Regexpf(t, l, res.Stdout, "stdout must contain: "+l)
	}

	for _, l := range opt.Unlike {
		assert.NotRegexpf(t, l, res.Stdout, "stdout must not contain: "+l)
	}

	if len(opt.ErrLike) == 0 {
		assert.Regexpf(t, `^\s*$`, res.Stderr, "stderr must be empty")
	} else {
		for _, l := range opt.ErrLike {
			assert.Regexpf(t, l, res.Stderr, "stderr contains: "+l)
		}
	}

	return res
}

func prepareCmd(ctx context.Context, opt *cmd) (check *exec.Cmd, outbuf, errbuf *bytes.Buffer) {
	check = exec.CommandContext(ctx, opt.Cmd, opt.Args...) //nolint:gosec // for testing purposes only
	check.Env = os.Environ()
	for key, val := range opt.Env {
		check.Env = append(check.Env, fmt.Sprintf("%s=%s", key, val))
	}
	switch opt.Dir {
	case "":
		workDir, _ := filepath.Abs(".")
		check.Dir = workDir
	default:
		check.Dir = opt.Dir
	}

	// byte buffer for output
	outbuf = &bytes.Buffer{}
	errbuf = &bytes.Buffer{}
	check.Stdout = outbuf
	check.Stderr = errbuf

	return check, outbuf, errbuf
}

func procWatch(ctx context.Context, proc *os.Process) {
	<-ctx.Done() // wait till command runs into timeout or is finished (canceled)
	if proc == nil {
		return
	}
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		// timeout
		_ = proc.Kill()
	case errors.Is(ctx.Err(), context.Canceled):
		// normal exit
		_ = proc.Kill()
	}
}

// getBinary returns path to snclient
func getBinary() string {
	workDir, _ := filepath.Abs(".")
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(workDir, "snclient.exe")
	default:
		return filepath.Join(workDir, "snclient")
	}
}

// logCmd prints some diagnostics useful when a command fails
func logCmd(t *testing.T, check *exec.Cmd, res *cmdResult) {
	t.Helper()
	t.Logf("cmd:     %s", check.String())
	t.Logf("path:    %s", check.Path)
	t.Logf("workdir: %s", check.Dir)
	t.Logf("exit:    %d", res.ExitCode)
	t.Logf("stdout:  %s", res.Stdout)
	t.Logf("stderr:  %s", res.Stderr)
}

// writeFile creates/updates a file with given content
func writeFile(t *testing.T, path, content string) {
	t.Helper()

	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoErrorf(t, err, "writing file %s succeeded", path)
}

// wait a couple of seconds till daemon answers
func waitUntilResponse(t *testing.T, bin string) {
	t.Helper()

	waitStart := time.Now()
	waitUntil := time.Now().Add(10 * time.Second)
	for time.Now().Before(waitUntil) {
		res := runCmd(t, &cmd{
			Cmd:  bin,
			Args: []string{"run", "check_nsc_web", "-k", "-p", "test", "-u", "https://localhost:8443", "check_snclient_version"},
			Exit: -1,
		})
		if res.ExitCode == 0 {
			if time.Since(waitStart) > 5*time.Second {
				t.Logf("daemon responded after %s", time.Since(waitStart))
			}

			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

var (
	daemonPid     = 0
	daemonPidFile = "snclient.lock"
	daemonCmdChan = make(chan *exec.Cmd, 1)
	daemonFinChan = make(chan bool, 1)
)

func startBackgroundDaemon(t *testing.T) {
	t.Helper()
	bin := getBinary()

	// start daemon
	go func() {
		res := runCmd(t, &cmd{ //nolint:testifylint // assertions outside of main goroutine secured by channel
			Cmd:        bin,
			Args:       []string{"-vv", "-logfile", "stdout", "-pidfile", daemonPidFile},
			Like:       []string{"starting", "listener on"},
			Unlike:     []string{"PANIC", `\[Error\]`},
			Exit:       -1,
			CmdChannel: daemonCmdChan,
		})
		require.NotNilf(t, res, "got daemon result") //nolint:testifylint // assertions outside of main goroutine secured by channel
		assert.NotContainsf(t, res.Stdout, "[Error]", "log does not contain errors")
		assert.Emptyf(t, res.Stderr, "stderr should be empty")
		daemonFinChan <- true
	}()

	startTimeOut := time.Now().Add(10 * time.Second)
	for time.Now().Before(startTimeOut) {
		pid, err := utils.ReadPid(daemonPidFile)
		if err == nil {
			daemonPid = pid

			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	require.Positivef(t, daemonPid, "daemon started")
}

func stopBackgroundDaemon(t *testing.T) bool {
	t.Helper()

	cmd := <-daemonCmdChan

	require.NotNilf(t, cmd, "daemon cmd should not be nil")

	err := cmd.Process.Kill()
	require.NoErrorf(t, err, "killing daemon")
	os.Remove(daemonPidFile)

	// wait till daemon tests are finished
	<-daemonFinChan

	return true
}

// returns time when daemon started, 0 if not available
func getStartedTime(t *testing.T, baseURL, localDaemonPassword string) float64 {
	t.Helper()

	res := runCmd(t, &cmd{
		Cmd:  "curl",
		Args: []string{"-s", "-u", "user:" + localDaemonPassword, "-k", baseURL + "/api/v1/inventory/uptime"},
		Exit: -1,
	})

	inventoryResult := struct {
		Snclient struct {
			Starttime float64 `json:"starttime"`
		} `json:"snclient"`
	}{}

	err := json.Unmarshal([]byte(res.Stdout), &inventoryResult)
	if err != nil {
		// that's ok, we wait for snclient to restart, so it might not be available yet
		t.Logf("request out: %s", res.Stdout)
		t.Logf("request err: %s", res.Stderr)
		t.Logf("request failed: %v", err)

		return 0
	}

	return inventoryResult.Snclient.Starttime
}
