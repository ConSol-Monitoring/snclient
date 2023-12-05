package snclient

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"pkg/convert"

	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_temperature"] = CheckEntry{"check_temperature", NewCheckTemperature}
}

type CheckTemperature struct {
	sensors []string
}

func NewCheckTemperature() CheckHandler {
	return &CheckTemperature{}
}

func (l *CheckTemperature) Build() *CheckData {
	return &CheckData{
		name:         "check_temperature",
		description:  "Check temperature sensors.",
		implemented:  Linux,
		hasInventory: ListInventory,
		args: map[string]CheckArgument{
			"sensor": {value: &l.sensors, isFilter: true, description: "Show this sensor only"},
		},
		result: &CheckResult{
			State: CheckExitOK,
		},
		defaultFilter:   "name=coretemp",
		defaultWarning:  "temperature > ${min} || temperature > ${crit}",
		defaultCritical: "temperature > ${min} || temperature > ${crit}",
		topSyntax:       "${status} - ${list}",
		detailSyntax:    "${label}: ${temperature:fmt=%.1f} °C",
		emptyState:      3,
		emptySyntax:     "check_temperature failed to find any sensors.",
		attributes: []CheckAttribute{
			{name: "name", description: "name of this sensor"},
			{name: "label", description: "label for this sensor"},
			{name: "path", description: "path to the sensor"},
			{name: "value", description: "current temperature"},
			{name: "crit", description: "critical value supplied from sensor"},
			{name: "max", description: "max value supplied from sensor"},
			{name: "min", description: "min value supplied from sensor"},
		},
		exampleDefault: `
    check_temperature
    OK - Package id 0: 65.0 °C, Core 0: 62.0 °C, Core 1: 61.0 °C, Core 2: 65.0 °C |...

Show all temperature sensors and apply custom thresholds:

    check_temperature filter=none warn="temperature > 85" crit="temperature > 90"
    OK - Package id 0: 65.0 °C, Core 0: 62.0 °C, Core 1: 61.0 °C, Core 2: 65.0 °C |...
	`,
	}
}

func (l *CheckTemperature) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("check_temperature is a linux only command")
	}

	hwmon, _ := filepath.Glob("/sys/class/hwmon/*/temp*_input")
	for _, dir := range hwmon {
		l.addHwMonFile(check, dir)
	}

	return check.Finalize()
}

func (l *CheckTemperature) addHwMonFile(check *CheckData, file string) {
	base := path.Dir(file)
	nameB, _ := os.ReadFile(base + "/name")
	name := strings.TrimSpace(string(nameB))

	entry := map[string]string{
		"name": name,
		"path": file,
	}

	prefix := strings.TrimSuffix(path.Base(file), "_input")

	labelB, _ := os.ReadFile(base + "/" + prefix + "_label")
	label := strings.TrimSpace(string(labelB))
	if label == "" {
		label = prefix
	}
	entry["label"] = label

	valueB, _ := os.ReadFile(file)
	temperature := convert.Float64(strings.TrimSpace(string(valueB))) / 1000
	entry["temperature"] = fmt.Sprintf("%f", temperature)

	critB, _ := os.ReadFile(base + "/" + prefix + "_crit")
	crit := convert.Float64(strings.TrimSpace(string(critB))) / 1000
	entry["crit"] = fmt.Sprintf("%f", crit)

	maxB, _ := os.ReadFile(base + "/" + prefix + "_max")
	max := convert.Float64(strings.TrimSpace(string(maxB))) / 1000
	entry["max"] = fmt.Sprintf("%f", max)

	minB, _ := os.ReadFile(base + "/" + prefix + "_min")
	min := convert.Float64(strings.TrimSpace(string(minB))) / 1000
	entry["min"] = fmt.Sprintf("%f", min)

	if len(l.sensors) > 0 && !slices.Contains(l.sensors, label) && !slices.Contains(l.sensors, name) {
		return
	}

	check.result.Metrics = append(check.result.Metrics, &CheckMetric{
		ThresholdName: label,
		Name:          label,
		Value:         temperature,
		Min:           &min,
		Max:           &max,
		Warning:       check.ExpandMetricMacros(check.TransformMultipleKeywords([]string{"temp", "temperature"}, label, check.warnThreshold), entry),
		Critical:      check.ExpandMetricMacros(check.TransformMultipleKeywords([]string{"temp", "temperature"}, label, check.critThreshold), entry),
	})

	check.listData = append(check.listData, entry)
}
