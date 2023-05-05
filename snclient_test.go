package snclient

import (
	"fmt"
	"os"
	"testing"
	"time"

	"pkg/utils"

	"github.com/stretchr/testify/assert"
)

func StartTestAgent(t *testing.T, config string, args []string) (osSignalChannel chan os.Signal, pidfile string, err error) {
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

	osSignalChannel = make(chan os.Signal, 1)

	go func() {
		osArgs := []string{
			fmt.Sprintf("--config=%s", tmpConfig.Name()),
			fmt.Sprintf("--pidfile=%s", tmpPidfile.Name()),
		}
		osArgs = append(osArgs, args...)
		SNClient("test", VERSION, osArgs, osSignalChannel)
	}()

	// wait for pid file
	waitDur := 10 * time.Second
	waitMax := time.Now().Add(waitDur)
	for {
		if time.Now().After(waitMax) {
			assert.Failf(t, "failed to start agent", "pidfile did not occur within %s", waitDur.String())

			return nil, "", fmt.Errorf("failed to start agent")
		}

		time.Sleep(50 * time.Millisecond)

		pid, err := utils.ReadPid(tmpPidfile.Name())
		if err != nil {
			continue
		}

		if pid > 0 {
			return osSignalChannel, tmpPidfile.Name(), nil
		}
	}
}

func StopTestAgent(t *testing.T, pidfile string, osSignalChannel chan os.Signal) {
	t.Helper()
	osSignalChannel <- os.Interrupt

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
