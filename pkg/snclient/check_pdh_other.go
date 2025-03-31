//go:build !windows

package snclient

import (
	"context"
	"fmt"
)

func (c *CheckPDH) check(_ context.Context, _ *Agent, _ *CheckData, _ []Argument) (*CheckResult, error) {
	return nil, fmt.Errorf("check_pdh is a windows only check")
}
