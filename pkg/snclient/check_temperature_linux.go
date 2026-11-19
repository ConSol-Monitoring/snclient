package snclient

import (
	"context"

	"github.com/shirou/gopsutil/v4/sensors"
)

func (l *CheckTemperature) mergeExclusiveSensors(ctx context.Context, src []sensors.TemperatureStat) ([]temperatureStat, error) {
	// reports min, lowest and highest, exclusive to linux.
	ex := sensors.NewExLinux()
	exSensors, err := ex.TemperatureWithContext(ctx)
	if err != nil {
		log.Debugf("(linux)sensors.TemperaturesWithContext: %s", err.Error())
	}

	minValueBySensorKey := make(map[string][]float64, len(exSensors))
	for idx := range exSensors {
		key := exSensors[idx].SensorKey
		minValueBySensorKey[key] = append(minValueBySensorKey[key], exSensors[idx].Min)
	}

	res := make([]temperatureStat, len(src))
	for idx := range src {
		minVal := float64(0)
		if values := minValueBySensorKey[src[idx].SensorKey]; len(values) > 0 {
			minVal = values[0]
			minValueBySensorKey[src[idx].SensorKey] = values[1:]
		}
		res[idx] = temperatureStat{
			TemperatureStat: src[idx],
			Min:             minVal,
		}
	}

	return res, nil
}
