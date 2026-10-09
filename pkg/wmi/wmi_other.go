//go:build !windows

package wmi

import (
	"fmt"
	"time"
)

func InitWbem() error {
	return nil
}

func QueryDefaultRetry(_ string, _ any) (err error) {
	return fmt.Errorf("requires windows os")
}

func QueryIsolatedDefaultRetry(_ string, _ any) (err error) {
	return fmt.Errorf("requires windows os")
}

func QueryIsolatedWithRetries(_ string, _ any, _ int, _ time.Duration) (err error) {
	return fmt.Errorf("requires windows os")
}

func QueryIsolated(_ string, _ any) (err error) {
	return fmt.Errorf("requires windows os")
}

func RawQuery(_ string) ([][]Data, error) {
	return nil, fmt.Errorf("requires windows os")
}
