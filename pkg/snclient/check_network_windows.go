package snclient

import (
	"fmt"

	"pkg/convert"
	"pkg/wmi"
)

func (l *CheckNetwork) interfaceSpeed(index int, name string) (speed int64, err error) {
	speed = -1

	query := fmt.Sprintf("SELECT InterfaceIndex, Name, Speed FROM Win32_NetworkAdapter WHERE InterfaceIndex = %d", index)
	result, err := wmi.Query(query)
	if err != nil {
		return speed, fmt.Errorf("wmi query failed: %s", err.Error())
	}

	if len(result) == 0 {
		return speed, fmt.Errorf("wmi query returned no data (interface %d / %s not found)", index, name)
	}

	for _, col := range result[0] {
		if col.Key == "Speed" {
			speedF, err := convert.Float64E(col.Value)
			if err != nil {
				return speed, fmt.Errorf("converting speed failed %s: %s", col.Value, err.Error())
			}
			speed = int64(speedF / 1e6)
		}
	}

	return speed, nil
}
