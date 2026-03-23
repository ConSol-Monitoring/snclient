//go:build !linux

package snclient

import (
	"context"
)

// execCommandAsRoot avoids compilation errors on non-Linux OSes where syscall.Credential
// is not available. It falls back to standard execution.
func (snc *Agent) execCommandAsRoot(ctx context.Context, command string, timeout int64) (stdout, stderr string, exitCode int64, err error) {
	return snc.execCommand(ctx, command, timeout)
}

// HasCapabilities returns false on non-Linux OSes.
func (snc *Agent) HasCapabilities() bool {
	return false
}
