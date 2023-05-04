package snclient

import (
	"fmt"
	"io/ioutil"
	"os"
	"pkg/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func StartTestAgent(t *testing.T, config string, args []string) error {
	tmpConfig, err := ioutil.TempFile("", "testconfig")
	assert.NoErrorf(t, err, "tmp config created")
	fmt.Fprintf(tmpConfig, config)
	err = tmpConfig.Close()
	assert.NoErrorf(t, err, "tmp config created")
	defer os.Remove(tmpConfig.Name())

	tmpPidfile, err := ioutil.TempFile("", "testpid")
	assert.NoErrorf(t, err, "tmp pidfile created")
	tmpPidfile.Close()
	os.Remove(tmpPidfile.Name())

	go func() {
		oldArgs := os.Args
		defer func() { os.Args = oldArgs }()
		os.Args = []string{"/usr/local/bin/snclient",
			fmt.Sprintf("--config=%s", tmpConfig.Name()),
			fmt.Sprintf("--pidfile=%s", tmpPidfile.Name()),
		}
		os.Args = append(os.Args, args...)
		SNClient("test", VERSION)
	}()

	// wait for pid file
	waitDur := 10 * time.Second
	waitMax := time.Now().Add(waitDur)
	for {
		if time.Now().After(waitMax) {
			assert.Failf(t, "failed to start agent", "pidfile did not occur within %s", waitDur.String())

			return fmt.Errorf("failed to start agent")
		}

		time.Sleep(50 * time.Millisecond)

		pid, err := utils.ReadPid(tmpPidfile.Name())
		if err != nil {
			continue
		}

		if pid > 0 {
			assert.NotNil(t, agent, "got global agent")

			return nil
		}
	}
}

func StopTestAgent(t *testing.T) {
	pidfile := agent.flags.flagPidfile
	agent.osSignalChannel <- os.Interrupt

	// wait for pid file to disapear
	waitDur := 10 * time.Second
	waitMax := time.Now().Add(waitDur)
	for {
		if time.Now().After(waitMax) {
			assert.Failf(t, "failed to stop agent", "pidfile still there after %s", waitDur.String())

			return
		}

		time.Sleep(50 * time.Millisecond)

		pid, err := utils.ReadPid(pidfile)
		if err != nil {
			return
		}

		_, err = os.FindProcess(pid)
		if err == nil {
			continue
		}

		return
	}
}
