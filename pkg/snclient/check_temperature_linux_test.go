package snclient

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/shirou/gopsutil/v4/sensors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemperature(t *testing.T) {
	snc := StartTestAgent(t, "")
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_temperature", []string{})
	out := string(res.BuildPluginOutput())
	if res.State == CheckExitUnknown && strings.Contains(out, "failed to find any sensors") {
		// no sensors found, cannot test
		return
	}
	assert.Regexpf(t, `^(OK|WARNING|CRITICAL|UNKNOWN) - .*: -?[\d.]+ °C`, out, "output matches")
	require.NotNil(t, res.Raw)
	assert.NotEqualf(t, CheckExitUnknown, res.State, "valid sensors must not make the check unknown")

	for _, entry := range res.Raw.listData {
		temp, err := strconv.ParseFloat(entry["temperature"], 64)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, temp, minPlausibleTemperature, "invalid sensor must be filtered out")
	}
}

func TestTemperatureMocked(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string
		expectState int64
		expectOut   string
		rejectOut   string
	}{
		{
			name: "plausible sensor",
			files: map[string]string{
				"class/hwmon/hwmon0/name":        "nvme",
				"class/hwmon/hwmon0/temp1_label": "Composite",
				"class/hwmon/hwmon0/temp1_input": "65000",
				"class/hwmon/hwmon0/temp1_max":   "69850",
			},
			expectState: CheckExitOK,
			expectOut:   "nvme_composite: 65.0 °C",
		},
		{
			name: "negative plausible sensor",
			files: map[string]string{
				"class/hwmon/hwmon0/name":        "nvme",
				"class/hwmon/hwmon0/temp1_label": "Composite",
				"class/hwmon/hwmon0/temp1_input": "-150",
			},
			expectState: CheckExitOK,
			expectOut:   "nvme_composite: -0.1 °C",
		},
		{
			name: "kelvin sentinel",
			files: map[string]string{
				"class/hwmon/hwmon0/name":        "nvme",
				"class/hwmon/hwmon0/temp1_label": "Composite",
				"class/hwmon/hwmon0/temp1_input": "-273150",
				"class/hwmon/hwmon0/temp1_max":   "69850",
			},
			expectState: CheckExitUnknown,
			expectOut:   "failed to find any sensors",
			rejectOut:   "-273.1",
		},
		{
			name: "temperature too high",
			files: map[string]string{
				"class/hwmon/hwmon0/name":        "nvme",
				"class/hwmon/hwmon0/temp1_label": "Composite",
				"class/hwmon/hwmon0/temp1_input": "200000",
			},
			expectState: CheckExitOK,
			expectOut:   "nvme_composite: 200.0 °C",
		},
		{
			name: "invalid and valid sensor",
			files: map[string]string{
				"class/hwmon/hwmon0/name":        "nvme",
				"class/hwmon/hwmon0/temp1_input": "-273150",
				"class/hwmon/hwmon1/name":        "coretemp",
				"class/hwmon/hwmon1/temp1_input": "65000",
			},
			expectState: CheckExitOK,
			expectOut:   "coretemp: 65.0 °C",
			rejectOut:   "-273.1",
		},
		{
			name: "thermal zones without hwmon",
			files: map[string]string{
				"class/thermal/thermal_zone0/type": "x86_pkg_temp",
				"class/thermal/thermal_zone0/temp": "65000",
				"class/thermal/thermal_zone1/type": "acpitz",
				"class/thermal/thermal_zone1/temp": "40000",
			},
			expectState: CheckExitOK,
			expectOut:   "x86_pkg_temp: 65.0 °C",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockSysFS(t, test.files)
			res := runTemperatureCheck(t)
			out := string(res.BuildPluginOutput())
			assert.Equal(t, test.expectState, res.State)
			assert.Contains(t, out, test.expectOut)
			if test.rejectOut != "" {
				assert.NotContains(t, out, test.rejectOut)
			}
		})
	}
}

func TestTemperatureSanitizeThresholds(t *testing.T) {
	check := NewCheckTemperature().(*CheckTemperature)
	data := check.Build()
	_, err := data.parseArgs(nil)
	require.NoError(t, err)

	check.addSensor(data, &temperatureStat{
		TemperatureStat: sensors.TemperatureStat{
			SensorKey:   "nvme_composite",
			Temperature: 65,
			Critical:    -273.15,
			High:        65261.85,
		},
		Min: -273.15,
	}, map[string]int{})
	check.addSensor(data, &temperatureStat{
		TemperatureStat: sensors.TemperatureStat{
			SensorKey:   "hot_sensor",
			Temperature: 65,
			Critical:    200,
			High:        200,
		},
		Min: 200,
	}, map[string]int{})

	require.Len(t, data.listData, 2)
	entry := data.listData[0]
	assert.Equal(t, "0.000000", entry["min"])
	assert.Equal(t, "0.000000", entry["crit"])
	assert.Equal(t, "0.000000", entry["max"])

	// There is no upper limit, so high threshold values stay.
	entry = data.listData[1]
	assert.Equal(t, "200.000000", entry["min"])
	assert.Equal(t, "200.000000", entry["crit"])
	assert.Equal(t, "200.000000", entry["max"])
}

func TestMergeExclusiveSensors(t *testing.T) {
	t.Run("min values", func(t *testing.T) {
		mockSysFS(t, map[string]string{
			"class/hwmon/hwmon0/name":        "nvme",
			"class/hwmon/hwmon0/temp1_label": "Composite",
			"class/hwmon/hwmon0/temp1_input": "65000",
			"class/hwmon/hwmon0/temp1_min":   "10000",
		})

		src, err := sensors.TemperaturesWithContext(context.Background())
		require.NoError(t, err)
		merged, err := (&CheckTemperature{}).mergeExclusiveSensors(context.Background(), src)
		require.NoError(t, err)
		require.Len(t, merged, 1)
		assert.InDelta(t, 10.0, merged[0].Min, 0.0001)
	})

	t.Run("duplicate keys", func(t *testing.T) {
		mockSysFS(t, map[string]string{
			"class/hwmon/hwmon0/name":        "nvme",
			"class/hwmon/hwmon0/temp1_label": "Composite",
			"class/hwmon/hwmon0/temp1_input": "65000",
			"class/hwmon/hwmon0/temp1_min":   "10000",
			"class/hwmon/hwmon1/name":        "nvme",
			"class/hwmon/hwmon1/temp1_label": "Composite",
			"class/hwmon/hwmon1/temp1_input": "66000",
			"class/hwmon/hwmon1/temp1_min":   "20000",
		})

		src, err := sensors.TemperaturesWithContext(context.Background())
		require.NoError(t, err)
		merged, err := (&CheckTemperature{}).mergeExclusiveSensors(context.Background(), src)
		require.NoError(t, err)
		require.Len(t, merged, 2)
		assert.InDelta(t, 10.0, merged[0].Min, 0.0001)
		assert.InDelta(t, 20.0, merged[1].Min, 0.0001)
	})

	t.Run("thermal zones without hwmon", func(t *testing.T) {
		mockSysFS(t, map[string]string{
			"class/thermal/thermal_zone0/type": "x86_pkg_temp",
			"class/thermal/thermal_zone0/temp": "65000",
			"class/thermal/thermal_zone1/type": "acpitz",
			"class/thermal/thermal_zone1/temp": "40000",
		})

		src, err := sensors.TemperaturesWithContext(context.Background())
		require.NoError(t, err)
		require.Len(t, src, 2)
		merged, err := (&CheckTemperature{}).mergeExclusiveSensors(context.Background(), src)
		require.NoError(t, err)
		require.Len(t, merged, 2)
		assert.Zero(t, merged[0].Min)
		assert.Zero(t, merged[1].Min)
	})
}

func mockSysFS(t *testing.T, files map[string]string) {
	t.Helper()
	sysDir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(sysDir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content+"\n"), 0o644))
	}
	t.Setenv("HOST_SYS", sysDir)
}

func runTemperatureCheck(t *testing.T) *CheckResult {
	t.Helper()
	check := NewCheckTemperature().(*CheckTemperature)
	data := check.Build()
	_, err := data.parseArgs(nil)
	require.NoError(t, err)
	res, err := check.Check(context.Background(), nil, data, nil)
	require.NoError(t, err)

	return res
}
