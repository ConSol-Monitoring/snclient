//go:build !windows

package snclient

import (
	"errors"
	"fmt"
)

// queryW32TimeStatus reports that the Windows Time service only exists on windows.
func queryW32TimeStatus() (w32timeStatus, error) {
	return w32timeStatus{}, fmt.Errorf("w32time.dll: %w", errors.ErrUnsupported)
}
