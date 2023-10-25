package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	DefaultCommandTimeout = 30 * time.Second
)

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

	// optional values when running a cmd
	Timeout time.Duration     // maximum run duration (default 30sec)
	Env     map[string]string // environment values
}

// runCmd runs a test command defined from the Cmd struct
func runCmd(t *testing.T, opt *cmd) {
	t.Helper()
	assert.NotEmptyf(t, opt.Cmd, "command must not be empty")

	if opt.Timeout <= 0 {
		opt.Timeout = DefaultCommandTimeout
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(opt.Timeout))
	defer cancel()

	check, outbuf, errbuf := prepareCmd(ctx, opt)

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
		Stdout: outbuf.String(),
		Stderr: errbuf.String(),
	}

	if err != nil && check.ProcessState == nil {
		logCmd(t, check, res)
		require.NoErrorf(t, err, fmt.Sprintf("command wait: %s", opt.Cmd))

		return
	}

	state := check.ProcessState

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		res.ExitCode = -1
		logCmd(t, check, res)
		assert.Fail(t, fmt.Sprintf("command run into timeout after %s", opt.Timeout.String()))

		return
	}

	if waitStatus, ok := state.Sys().(syscall.WaitStatus); ok {
		res.ExitCode = int64(waitStatus.ExitStatus())
	}

	if opt.Exit != -1 {
		assert.Equalf(t, opt.Exit, res.ExitCode, fmt.Sprintf("exit code is: %d", opt.Exit))
	}

	for _, l := range opt.Like {
		assert.Regexpf(t, l, res.Stdout, "stdout contains: "+l)
	}

	if len(opt.ErrLike) == 0 {
		assert.Regexpf(t, `^\s*$`, res.Stderr, "stderr must be empty")
	} else {
		for _, l := range opt.ErrLike {
			assert.Regexpf(t, l, res.Stderr, "stderr contains: "+l)
		}
	}
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
	require.NoErrorf(t, err, fmt.Sprintf("writing file %s succeeded", path))
}
