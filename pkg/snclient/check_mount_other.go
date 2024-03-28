//go:build !windows

package snclient

import (
	"context"
)

func (l *CheckMount) getVolumes(_ context.Context, _ map[string]bool) (drives []map[string]string, err error) {
	return drives, nil
}
