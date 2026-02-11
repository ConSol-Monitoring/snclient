package snclient

import (
	"context"

	"github.com/shirou/gopsutil/v4/sensors"
)

func (l *CheckTemperature) mergeExclusiveSensors(ctx context.Context, src []sensors.TemperatureStat) ([]temperatureStat, error) {
	ex := sensors.NewExLinux()
	exSensors, err := ex.TemperatureWithContext(ctx)
	if err != nil {
		log.Debugf("(linux)sensors.TemperaturesWithContext: %s", err.Error())
	}

	res := make([]temperatureStat, len(src))
	for idx := range src {
		minVal := float64(0)
		if len(exSensors) < idx {
			minVal = exSensors[idx].Min
		}
		sens := temperatureStat{
			TemperatureStat: src[idx],
			Min:             minVal,
		}
		res[idx] = sens
	}

	return res, nil
}
