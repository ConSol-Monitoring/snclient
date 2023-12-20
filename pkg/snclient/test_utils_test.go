package snclient

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	setLogLevel("error")
}

// Starts a full Agent from given config
func StartTestAgent(t *testing.T, config string) *Agent {
	t.Helper()
	testDefaultConfig := `
[/modules]
WEBServer = disabled
`
	tmpConfig, err := os.CreateTemp("", "testconfig")
	require.NoErrorf(t, err, "tmp config created")
	_, err = tmpConfig.WriteString(testDefaultConfig)
	require.NoErrorf(t, err, "tmp defaults written")
	_, err = tmpConfig.WriteString(config)
	require.NoErrorf(t, err, "tmp config written")
	err = tmpConfig.Close()
	require.NoErrorf(t, err, "tmp config created")
	defer os.Remove(tmpConfig.Name())

	tmpPidfile, err := os.CreateTemp("", "testpid")
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
		newTmp, err := os.MkdirTemp("", "testtmp*")
		require.NoErrorf(t, err, "mkdirTemp worked")

		newPath := append([]string{newTmp}, pathElements...)
		err = os.Setenv("PATH", strings.Join(newPath, ":"))
		require.NoErrorf(t, err, "set env worked")
		tmpPath = newTmp
	}

	// make sure folder is still there
	err := os.MkdirAll(tmpPath, 0o700)
	require.NoErrorf(t, err, "mkdir worked")

	// add scripts
	for key, data := range utils {
		scriptData := []string{"#!/bin/sh"}
		scriptData = append(scriptData, "cat << EOT", data, "EOT")
		err := os.WriteFile(filepath.Join(tmpPath, key), []byte(strings.Join(scriptData, "\n")), 0o600)
		require.NoErrorf(t, err, "writing %s worked", filepath.Join(tmpPath, key))
		err = os.Chmod(filepath.Join(tmpPath, key), 0o700)
		require.NoErrorf(t, err, "chmod %s worked", filepath.Join(tmpPath, key))
	}

	return tmpPath
}
