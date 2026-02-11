//go:build linux || windows || darwin

package snclient

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/utils"
	"github.com/shirou/gopsutil/v4/sensors"
)

func init() {
	AvailableChecks["check_temperature"] = CheckEntry{"check_temperature", NewCheckTemperature}
}

type CheckTemperature struct {
	sensors []string
}

// enhanced psutil temperatureStat with min value
type temperatureStat struct {
	sensors.TemperatureStat
	Min float64 // Temperature min value.
}

func NewCheckTemperature() CheckHandler {
	return &CheckTemperature{}
}

func (l *CheckTemperature) Build() *CheckData {
	return &CheckData{
		name:         "check_temperature",
		description:  "Check temperature sensors.",
		implemented:  Linux | Windows | Darwin,
		hasInventory: ListInventory,
		args: map[string]CheckArgument{
			"sensor": {value: &l.sensors, isFilter: true, description: "Show this sensor only"},
		},
		result: &CheckResult{
			State: CheckExitOK,
		},
		defaultFilter:   "temperature != 0 and temperature != 1", // seems like disabled sensors return 1.0000 or 0.0000
		defaultWarning:  "temperature < ${min} || temperature > ${crit}",
		defaultCritical: "temperature < ${min} || temperature > ${crit}",
		topSyntax:       "${status} - ${list}",
		detailSyntax:    "${sensor}: ${temperature:fmt=%.1f} °C",
		emptyState:      3,
		emptySyntax:     "check_temperature failed to find any sensors.",
		attributes: []CheckAttribute{
			{name: "sensor", description: "full name of this sensor, ex.: coretemp_core_0"},
			{name: "name", description: "name of this sensor, ex.: coretemp"},
			{name: "label", description: "label for this sensor, ex.: core 0"},
			{name: "value", description: "current temperature"},
			{name: "crit", description: "critical value supplied from sensor"},
			{name: "max", description: "max value supplied from sensor"},
			{name: "min", description: "min value supplied from sensor"},
		},
		listSorted: []string{"label"},
		exampleDefault: `
    check_temperature
    OK - Package id 0: 65.0 °C, Core 0: 62.0 °C, Core 1: 61.0 °C, Core 2: 65.0 °C |...

Show all temperature sensors and apply custom thresholds:

    check_temperature filter=none warn="temperature > 85" crit="temperature > 90"
    OK - Package id 0: 65.0 °C, Core 0: 62.0 °C, Core 1: 61.0 °C, Core 2: 65.0 °C |...
	`,
	}
}

func (l *CheckTemperature) Check(ctx context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	sens, err := sensors.TemperaturesWithContext(ctx)
	if err != nil {
		log.Debugf("sensors.TemperaturesWithContext: %s", err.Error())
	}
	merged, err := l.mergeExclusiveSensors(ctx, sens)
	if err != nil {
		log.Debugf("os specific sensors error: %s", err.Error())
	}

	duplicates := map[string]int{}
	for i := range merged {
		l.addSensor(check, &merged[i], duplicates)
	}

	return check.Finalize()
}

func (l *CheckTemperature) addSensor(check *CheckData, sensor *temperatureStat, duplicates map[string]int) {
	fields := utils.FieldsN(strings.ReplaceAll(sensor.SensorKey, "_", " "), 2)
	name := fields[0]
	label := fields[0]
	if len(fields) >= 2 {
		label = fields[1]
	}
	sensorKey := sensor.SensorKey
	if num, ok := duplicates[label]; ok {
		duplicates[label] = num + 1
		label = fmt.Sprintf("%s.%d", label, num)
		sensorKey = fmt.Sprintf("%s.%d", sensorKey, num)
	} else {
		duplicates[label] = 1
	}
	entry := map[string]string{
		"sensor":      sensorKey,
		"name":        name,
		"label":       label,
		"temperature": fmt.Sprintf("%f", sensor.Temperature),
		"crit":        fmt.Sprintf("%f", sensor.Critical),
		"max":         fmt.Sprintf("%f", sensor.High),
		"min":         fmt.Sprintf("%f", sensor.Min),
	}

	if len(l.sensors) > 0 && !slices.Contains(l.sensors, name) && !slices.Contains(l.sensors, label) {
		return
	}

	if !check.MatchMapCondition(check.filter, entry, true) {
		return
	}

	check.result.Metrics = append(check.result.Metrics, &CheckMetric{
		ThresholdName: sensorKey,
		Name:          sensorKey,
		Value:         sensor.Temperature,
		Min:           &sensor.Min,
		Max:           &sensor.High,
		Warning:       check.ExpandMetricMacros(check.TransformMultipleKeywords([]string{"temp", "temperature"}, sensorKey, check.warnThreshold), entry),
		Critical:      check.ExpandMetricMacros(check.TransformMultipleKeywords([]string{"temp", "temperature"}, sensorKey, check.critThreshold), entry),
	})

	check.listData = append(check.listData, entry)
}
