//go:build !windows

package snclient

import (
	"context"
	"fmt"
)

func (l *CheckPing) addPingWindows(_ context.Context, _ *CheckData) error {
	return fmt.Errorf("ping is not supported on this platform")
}
