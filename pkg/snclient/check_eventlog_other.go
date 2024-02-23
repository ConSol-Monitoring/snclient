//go:build !windows

package snclient

import (
	"context"
	"fmt"
	"runtime"
)

func (l *CheckEventlog) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	return nil, fmt.Errorf("not implemented on platform %s",runtime.GOOS)
}
