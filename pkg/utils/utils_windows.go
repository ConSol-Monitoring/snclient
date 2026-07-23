package utils

import (
	"fmt"
	"os"
	"path/filepath"
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

	file, err := os.CreateTemp(path, ".writable-*")
	if err != nil {
		return fmt.Errorf("directory is not writable: %w", err)
	}

	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)

	return nil
}
