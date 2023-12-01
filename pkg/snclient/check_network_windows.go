package snclient

import (
	"fmt"

	"pkg/convert"
	"pkg/wmi"
)

func (l *CheckNetwork) interfaceSpeed(index int, name string) (speed int64, err error) {
	speed = -1

	query := fmt.Sprintf("SELECT InterfaceIndex, Name, Speed FROM Win32_NetworkAdapter WHERE InterfaceIndex = %d", index)
	result, err := wmi.QueryDefaultRetry(query)
	if err != nil {
		return speed, fmt.Errorf("wmi query failed: %s", err.Error())
	}

	if len(result) == 0 {
		return speed, fmt.Errorf("wmi query returned no data (interface %d / %s not found)", index, name)
	}

	val, ok := result[0]["Speed"]
	if !ok {
		return speed, fmt.Errorf("wmi query returned no speed column for interface %d / %s", index, name)
	}

	speed, err = convert.Int64E(val)
	if err != nil {
		return speed, fmt.Errorf("converting speed failed %s: %s", val, err.Error())
	}
	speed /= 1e6

	return speed, nil
}
