//go:build !windows

package wmi

import "fmt"

func InitWbem() error {
	return nil
}

func QueryDefaultRetry(_ string, _ interface{}) (err error) {
	return fmt.Errorf("requires windows os")
}

func RawQuery(_ string) ([][]Data, error) {
	return nil, fmt.Errorf("requires windows os")
}
