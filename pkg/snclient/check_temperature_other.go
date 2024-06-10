//go:build windows || darwin

package snclient

import (
	"context"

	"github.com/shirou/gopsutil/v4/sensors"
)

func (l *CheckTemperature) mergeExclusiveSensors(_ context.Context, src []sensors.TemperatureStat) ([]temperatureStat, error) {
	res := make([]temperatureStat, len(src))
	for i := range src {
		sens := temperatureStat{
			TemperatureStat: src[i],
			Min:             0,
		}
		res[i] = sens
	}

	return res, nil
}
