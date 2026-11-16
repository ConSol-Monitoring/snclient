package snclient

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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
	assert.Regexpf(t, `^(OK|UNKNOWN) - .*: -?[\d.]+ °C`, out, "output matches")
	require.NotNil(t, res.Raw)

	hasImplausible := false
	for _, entry := range res.Raw.listData {
		temp, err := strconv.ParseFloat(entry["temperature"], 64)
		require.NoError(t, err)
		if temp < minPlausibleTemperature || temp > maxPlausibleTemperature {
			hasImplausible = true
		}
	}
	if hasImplausible {
		assert.Equalf(t, CheckExitUnknown, res.State, "implausible sensor must make the check unknown")
	} else {
		assert.NotEqualf(t, CheckExitUnknown, res.State, "plausible sensors must not make the check unknown")
	}
}

func TestTemperatureMocked(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string
		expectState int64
		expectOut   string
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
			name: "kelvin sentinel",
			files: map[string]string{
				"class/hwmon/hwmon0/name":        "nvme",
				"class/hwmon/hwmon0/temp1_label": "Composite",
				"class/hwmon/hwmon0/temp1_input": "-273150",
				"class/hwmon/hwmon0/temp1_max":   "69850",
			},
			expectState: CheckExitUnknown,
			expectOut:   "nvme_composite: -273.1 °C",
		},
		{
			name: "temperature too high",
			files: map[string]string{
				"class/hwmon/hwmon0/name":        "nvme",
				"class/hwmon/hwmon0/temp1_label": "Composite",
				"class/hwmon/hwmon0/temp1_input": "200000",
			},
			expectState: CheckExitUnknown,
			expectOut:   "nvme_composite: 200.0 °C",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockSysFS(t, test.files)
			res := runTemperatureCheck(t)
			assert.Equal(t, test.expectState, res.State)
			assert.Contains(t, string(res.BuildPluginOutput()), test.expectOut)
		})
	}
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
