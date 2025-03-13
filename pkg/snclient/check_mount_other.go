//go:build !windows

package snclient

import (
	"context"
)

func (l *CheckMount) getVolumes(_ context.Context, _ *CheckData, _ map[string]bool) (drives []map[string]string, err error) {
	return drives, nil
}
