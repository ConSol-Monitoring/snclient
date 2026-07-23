//go:build !windows

package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// IsWritable returns true if given folder is writable
func IsWritable(path string) error {
	path = filepath.Join(path, ".")
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist: %s", path, err.Error())
	}
	if err != nil {
		return fmt.Errorf("stat %s: %s", path, err.Error())
	}
	if !stat.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}

	err = unix.Access(path, unix.W_OK)
	if err != nil {
		return fmt.Errorf("access failed on %s: %s", path, err.Error())
	}

	return nil
}
