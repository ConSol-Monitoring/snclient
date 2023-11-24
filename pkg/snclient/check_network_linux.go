package snclient

import (
	"fmt"
	"os"
	"strings"

	"pkg/convert"
)

func (l *CheckNetwork) interfaceSpeed(_ int, name string) (int64, error) {
	// grab speed from /sys/class/net/<dev>/speed if possible
	procFile := fmt.Sprintf("/sys/class/net/%s/speed", name)
	dat, err := os.ReadFile(procFile)
	if err != nil {
		return -1, fmt.Errorf("failed to read %s: %s", procFile, err.Error())
	}
	speed, err := convert.Int64E(strings.TrimSpace(string(dat)))
	if err != nil {
		return -1, fmt.Errorf("failed to convert to int %s: %s", dat, err.Error())
	}

	return speed, nil
}
