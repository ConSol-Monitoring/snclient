package snclient

import (
	"fmt"
	"testing"
	"time"

	"github.com/consol-monitoring/snclient/pkg/humanize"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessMemoryWatcherConfigParsing(t *testing.T) {
	section := NewConfig(true).Section("/settings/process memory")
	section.Set("memory limit", "32MB")
	section.Set("check interval", "250ms")

	handler := &ProcessMemoryWatcherHandler{}
	err := handler.Init(NewAgentSimple(&AgentFlags{}), section, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, uint64(32000000), handler.memoryLimit)
	assert.Equal(t, 250*time.Millisecond, handler.checkInterval)
}

func TestProcessMemoryWatcherStartDisabledOnZeroLimit(t *testing.T) {
	section := NewConfig(true).Section("/settings/process memory")
	section.Set("memory limit", "0")
	section.Set("check interval", "1s")

	handler := &ProcessMemoryWatcherHandler{}
	err := handler.Init(NewAgentSimple(&AgentFlags{}), section, nil, nil)
	require.NoError(t, err)

	err = handler.Start()
	require.NoError(t, err)
	assert.False(t, handler.running.Load())
}

func TestProcessMemoryWatcherExceedsLimitPanics(t *testing.T) {
	disableLogsTemporarily()
	defer restoreLogLevel()

	handler := &ProcessMemoryWatcherHandler{
		memoryLimit:   1,
		checkInterval: 100 * time.Millisecond,
	}
	handler.stopChannel = make(chan bool)

	defer func() {
		recovered := recover()
		require.NotNil(t, recovered)
		assert.Contains(t, fmt.Sprint(recovered), "process memory limit exceeded")
	}()

	handler.mainLoop()
}

func TestProcessMemoryWatcherCurrentRSS(t *testing.T) {
	handler := &ProcessMemoryWatcherHandler{}
	rss, err := handler.currentRSS()
	require.NoError(t, err)
	assert.Positive(t, rss)

	// sanity format check for the log formatter path used in the watcher
	assert.NotEmpty(t, humanize.BytesF(rss, 2))
}
