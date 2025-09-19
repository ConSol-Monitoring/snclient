//go:build !windows

package snclient

import (
	"fmt"
)

func (l *CheckPagefile) check(_ *CheckData) (*CheckResult, error) {
	return nil, fmt.Errorf("check_pagefile is a windows only check")
}
