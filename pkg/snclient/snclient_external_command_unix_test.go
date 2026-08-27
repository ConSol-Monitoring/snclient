//go:build !windows

package snclient

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunExternalCommandAcceptsSubsecondTimeout(t *testing.T) {
	snc := NewAgentSimple(&AgentFlags{})
	snc.config.Section("/paths").Set("shared-path", t.TempDir())
	cmd, err := snc.MakeCmd(context.Background(), "sleep 10")
	require.NoError(t, err)

	started := time.Now()
	_, _, _, _, err = snc.runExternalCommand(context.Background(), cmd, 50*time.Millisecond) //nolint:dogsled // it is just a test

	require.ErrorContains(t, err, "timeout")
	require.Less(t, time.Since(started), 2*time.Second)
}
