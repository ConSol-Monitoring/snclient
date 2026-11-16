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

	// Keep the values in order. Some systems expose the same sensor key more than once.
	minByKey := make(map[string][]float64, len(exSensors))
	for idx := range exSensors {
		key := exSensors[idx].SensorKey
		minByKey[key] = append(minByKey[key], exSensors[idx].Min)
	}

	res := make([]temperatureStat, len(src))
	for idx := range src {
		minVal := float64(0)
		if values := minByKey[src[idx].SensorKey]; len(values) > 0 {
			minVal = values[0]
			minByKey[src[idx].SensorKey] = values[1:]
		}
		res[idx] = temperatureStat{
			TemperatureStat: src[idx],
			Min:             minVal,
		}
	}

	return res, nil
}
