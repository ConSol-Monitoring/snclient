package snclient

import (
	"fmt"
	"os"
	"strings"

	"pkg/convert"
)

func (l *CheckNetwork) interfaceSpeed(_ int, name string) (speed int64, err error) {
	speed = -1

	// grab speed from /sys/class/net/<dev>/speed if possible
	procFile := fmt.Sprintf("/sys/class/net/%s/speed", name)
	dat, err := os.ReadFile(procFile)
	if err != nil {
		return speed, fmt.Errorf("failed to read %s: %s", procFile, err.Error())
	}
	speedF, err := convert.Float64E(strings.TrimSpace(string(dat)))
	if err != nil {
		return speed, fmt.Errorf("failed to convert to int %s: %s", dat, err.Error())
	}

	return int64(speedF), nil
}
