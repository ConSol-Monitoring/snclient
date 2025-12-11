//go:build freebsd

package snclient

import (
	"context"
	"fmt"
	"runtime"
)

func (l *CheckOSUpdates) addOSBackends(_ context.Context, _ *CheckData) (int, error) {
	return 0, fmt.Errorf("os update backend not yet implemented, runtime.GOOS: %s", runtime.GOOS)
}
