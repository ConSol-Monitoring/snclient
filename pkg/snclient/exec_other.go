//go:build !linux

package snclient

import (
	"context"
	"time"
)

// execCommandAsRoot avoids compilation errors on non-Linux OSes where syscall.Credential
// is not available. It falls back to standard execution.
func (snc *Agent) execCommandAsRoot(ctx context.Context, command string, timeout time.Duration) (stdout, stderr string, exitCode int64, err error) {
	return snc.execCommand(ctx, command, timeout)
}

// HasCapabilities returns false on non-Linux OSes.
func HasCapabilities() bool {
	if testModeFakeHasCapabilities {
		log.Debug("has capabilities override enabled for testing")

		return true
	}

	return false
}
