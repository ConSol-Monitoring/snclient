package snclient

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	level := "error"
	// set log level from env
	env := os.Getenv("SNCLIENT_VERBOSE")
	switch env {
	case "":
	case "1":
		level = "verbose"
	case "2":
		level = "trace"
	default:
		level = env
	}
	setLogLevel(level)
}

var testAgentStarted = atomic.Bool{}

// Starts a full Agent from given config
func StartTestAgent(t *testing.T, config string) *Agent {
	t.Helper()

	if testAgentStarted.Load() {
		t.Fatalf("test agent already started, forgot to call StopTestAgent()?")
	}
	testAgentStarted.Store(true)
	testDefaultConfig := `
[/modules]
WEBServer = disabled
`
	tmpConfig, err := os.CreateTemp(t.TempDir(), "testconfig")
	require.NoErrorf(t, err, "tmp config created")
	_, err = tmpConfig.WriteString(testDefaultConfig)
	require.NoErrorf(t, err, "tmp defaults written")
	_, err = tmpConfig.WriteString(config)
	require.NoErrorf(t, err, "tmp config written")
	err = tmpConfig.Close()
	require.NoErrorf(t, err, "tmp config created")
	defer os.Remove(tmpConfig.Name())

	tmpPidfile, err := os.CreateTemp(t.TempDir(), "testpid")
	require.NoErrorf(t, err, "tmp pidfile created")
	tmpPidfile.Close()
	os.Remove(tmpPidfile.Name())

	flags := &AgentFlags{
		Quiet:       true,
		ConfigFiles: []string{tmpConfig.Name()},
		Pidfile:     tmpPidfile.Name(),
		Mode:        ModeServer,
	}
	snc := NewAgent(flags)
	started := snc.StartWait(10 * time.Second)
	assert.Truef(t, started, "agent is started successfully")
	if !started {
		t.Fatalf("agent did not start")
	}

	return snc
}

// Stops the agent started by StartTestAgent
func StopTestAgent(t *testing.T, snc *Agent) {
	t.Helper()

	stopped := snc.StopWait(10 * time.Second)
	assert.Truef(t, stopped, "agent stopped successfully")
	if !stopped {
		t.Fatalf("agent did not stop")
	}
	testAgentStarted.Store(false)
}

// mock utilities in a tmp path
// key is the filename of the util and the data is the text returned by this script
func MockSystemUtilities(t *testing.T, utils map[string]string) (tmpPath string) {
	t.Helper()

	curPath := os.Getenv("PATH")
	pathElements := strings.Split(curPath, ":")
	tmpPath = pathElements[0]

	// if first element is not a tmp path already, create one
	if !strings.Contains(tmpPath, "/testtmp") {
		newTmp := t.TempDir()

		newPath := append([]string{newTmp}, pathElements...)
		t.Setenv("PATH", strings.Join(newPath, ":"))
		tmpPath = newTmp
	}

	// make sure folder is still there
	err := os.MkdirAll(tmpPath, 0o700)
	require.NoErrorf(t, err, "mkdir worked")

	// add scripts
	for key, data := range utils {
		if strings.HasSuffix(key, "_exit") {
			continue
		}
		scriptData := []string{"#!/bin/sh"}
		scriptData = append(scriptData, "cat << EOT", data, "EOT")
		if exitCode, ok := utils[key+"_exit"]; ok {
			scriptData = append(scriptData, fmt.Sprintf("exit %s;", exitCode))
		}
		err := os.WriteFile(filepath.Join(tmpPath, key), []byte(strings.Join(scriptData, "\n")), 0o600)
		require.NoErrorf(t, err, "writing %s worked", filepath.Join(tmpPath, key))
		err = os.Chmod(filepath.Join(tmpPath, key), 0o700)
		require.NoErrorf(t, err, "chmod %s worked", filepath.Join(tmpPath, key))
	}

	return tmpPath
}

// getRandomFreeTCPPort returns a random free tcp port
func getRandomFreeTCPPort(t *testing.T) (port int) {
	t.Helper()

	// get a random port by opening a listener on port 0 and then closing it
	listen, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoErrorf(t, err, "get free listener worked")

	addr := listen.Addr()
	require.IsTypef(t, &net.TCPAddr{}, addr, "listener address is TCP")
	if addr, ok := listen.Addr().(*net.TCPAddr); ok {
		port = addr.Port
	}
	err = listen.Close()
	require.NoErrorf(t, err, "close free listener worked")

	return port
}

// wait for http ok and given text
func waitForStatusOKText(t *testing.T, url, text string) string {
	t.Helper()
	for range 300 {
		body := waitForStatusOK(t, url)
		if strings.Contains(body, text) {
			return body
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s to contain %q", url, text)

	return ""
}

func waitForStatusOK(t *testing.T, url string) string {
	t.Helper()

	var lastErr error
	for range 300 {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
		require.NoErrorf(t, err, "request created")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(100 * time.Millisecond)

			continue
		}

		if res.StatusCode == http.StatusOK {
			body, hErr := io.ReadAll(res.Body)
			require.NoError(t, hErr)
			res.Body.Close()

			return string(body)
		}
		res.Body.Close()
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s: %v", url, lastErr)

	return ""
}
