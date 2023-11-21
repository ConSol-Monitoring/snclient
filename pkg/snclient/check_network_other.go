//go:build !windows && !linux

package snclient

import (
	"fmt"
	"runtime"
)

func (l *CheckNetwork) interfaceSpeed(_ int, _ string) (int64, error) {
	return -1, fmt.Errorf("interface speed not supported on %s", runtime.GOOS)
}
