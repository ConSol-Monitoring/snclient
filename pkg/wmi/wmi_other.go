//go:build !windows

package wmi

import (
	"fmt"

	"github.com/kdar/factorlog"
)

func InitWbem(_ *factorlog.FactorLog, _ int) error {
	return nil
}

func QueryDefaultRetry(_ string, _ any) (err error) {
	return fmt.Errorf("requires windows os")
}

func RawQuery(_, _ string) ([][]Data, error) {
	return nil, fmt.Errorf("requires windows os")
}
