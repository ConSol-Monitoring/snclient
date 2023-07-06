package snclient

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func init() {
	setLogLevel("error")
}

// Starts a full Agent from given config
func StartTestAgent(t *testing.T, config string) *Agent {
	t.Helper()
	tmpConfig, err := os.CreateTemp("", "testconfig")
	assert.NoErrorf(t, err, "tmp config created")
	_, err = tmpConfig.WriteString(config)
	assert.NoErrorf(t, err, "tmp config written")
	err = tmpConfig.Close()
	assert.NoErrorf(t, err, "tmp config created")
	defer os.Remove(tmpConfig.Name())

	tmpPidfile, err := os.CreateTemp("", "testpid")
	assert.NoErrorf(t, err, "tmp pidfile created")
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

func TestPasswords(t *testing.T) {
	config := fmt.Sprintf(`
[/settings]
password0 =
password1 = %s
password2 = secret
password3 = SHA256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08
`, DefaultPassword)

	snc := StartTestAgent(t, config)
	conf := snc.Config.Section("/settings")

	disableLogsTemporarily()
	defer restoreLogLevel()

	p0, _ := conf.GetString("password0")
	assert.Equalf(t, true, snc.verifyPassword(p0, "test"), "password check disabled -> ok")
	assert.Equalf(t, true, snc.verifyPassword(p0, ""), "password check disabled -> ok")

	p1, _ := conf.GetString("password1")
	assert.Equalf(t, false, snc.verifyPassword(p1, "test"), "default password still set -> not ok")
	assert.Equalf(t, false, snc.verifyPassword(p1, DefaultPassword), "default password still set -> not ok")

	p2, _ := conf.GetString("password2")
	assert.Equalf(t, true, snc.verifyPassword(p2, "secret"), "simple password -> ok")
	assert.Equalf(t, false, snc.verifyPassword(p2, "wrong"), "simple password wrong")

	p3, _ := conf.GetString("password3")
	assert.Equalf(t, true, snc.verifyPassword(p3, "test"), "hashed password -> ok")
	assert.Equalf(t, false, snc.verifyPassword(p3, "wrong"), "hashed password wrong")

	StopTestAgent(t, snc)
}
