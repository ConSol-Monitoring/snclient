package snclient

import (
	"fmt"

	"pkg/wmi"
)

func (l *CheckNetwork) interfaceSpeed(index int, name string) (speed int64, err error) {
	speed = -1

	// https://learn.microsoft.com/en-us/windows/win32/cimwin32prov/win32-networkadapter
	interfaces := []struct {
		InterfaceIndex string
		Name           string
		Speed          uint64
	}{}
	query := fmt.Sprintf("SELECT InterfaceIndex, Name, Speed FROM Win32_NetworkAdapter WHERE InterfaceIndex = %d", index)
	err = wmi.QueryDefaultRetry(query, &interfaces)
	if err != nil {
		return speed, fmt.Errorf("wmi query failed: %s", err.Error())
	}

	if len(interfaces) == 0 {
		return speed, fmt.Errorf("wmi query returned no data (interface %d / %s not found)", index, name)
	}

	speed = int64(interfaces[0].Speed) / 1e6

	return speed, nil
}
