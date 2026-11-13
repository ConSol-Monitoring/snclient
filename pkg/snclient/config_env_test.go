package snclient

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func unsetEnv(t *testing.T, name string) {
	t.Helper()

	old, hadValue := os.LookupEnv(name)
	require.NoError(t, os.Unsetenv(name))

	if hadValue {
		t.Cleanup(func() {
			t.Setenv(name, old)
		})
	}
}

func TestReplaceEnvMacros(t *testing.T) {
	t.Setenv("SNCLIENT_TEST_ENV", "value")
	t.Setenv("SNCLIENT_TEST_ENV_EMPTY", "")
	t.Setenv("SNCLIENT_TEST_ENV_NESTED", "$ENV{SNCLIENT_TEST_ENV}")
	t.Setenv("SNCLIENT_TEST_ENV_PARENS", "pa$(x)")
	unsetEnv(t, "SNCLIENT_TEST_ENV_UNSET")

	for _, test := range []struct {
		name     string
		input    string
		expected string
		err      string
	}{
		{"plain", "$ENV{SNCLIENT_TEST_ENV}", "value", ""},
		{"embedded", "prefix-$ENV{SNCLIENT_TEST_ENV}-suffix", "prefix-value-suffix", ""},
		{"multiple", "$ENV{SNCLIENT_TEST_ENV}/$ENV{SNCLIENT_TEST_ENV}", "value/value", ""},
		{"quoted", `"$ENV{SNCLIENT_TEST_ENV}"`, `"value"`, ""},
		{"empty", "[$ENV{SNCLIENT_TEST_ENV_EMPTY}]", "[]", ""},
		{"spaces", "$ENV{ SNCLIENT_TEST_ENV }", "value", ""},
		{"escape", "$$ENV{SNCLIENT_TEST_ENV}", "$ENV{SNCLIENT_TEST_ENV}", ""},
		{"unclosed", "$ENV{SNCLIENT_TEST_ENV", "$ENV{SNCLIENT_TEST_ENV", ""},
		{"nested", "$ENV{SNCLIENT_TEST_ENV_NESTED}", "$ENV{SNCLIENT_TEST_ENV}", ""},
		{"macro in value", "$ENV{SNCLIENT_TEST_ENV_PARENS}", "pa$(x)", ""},
		{"unset", "$ENV{SNCLIENT_TEST_ENV_UNSET}", "$ENV{SNCLIENT_TEST_ENV_UNSET}", `environment variable "SNCLIENT_TEST_ENV_UNSET" is not set`},
		{"empty name", "$ENV{}", "$ENV{}", "empty environment variable name"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := ReplaceEnvMacros(test.input)
			if test.err != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestConfigEnvMacros(t *testing.T) {
	t.Setenv("SNCLIENT_TEST_ENV_HOST", "10.1.2.3")
	t.Setenv("SNCLIENT_TEST_ENV_PASSWORD", "s3cret")
	t.Setenv("SNCLIENT_TEST_ENV_EMPTY", "")

	configText := `
[/settings/default]
allowed hosts = 127.0.0.1, $ENV{SNCLIENT_TEST_ENV_HOST}
referenced = $ENV{SNCLIENT_TEST_ENV_HOST}

[/settings/WEB/server]
password = "$ENV{SNCLIENT_TEST_ENV_PASSWORD}"

[/test]
combined = prefix-$ENV{SNCLIENT_TEST_ENV_HOST}-suffix
empty = before-$ENV{SNCLIENT_TEST_ENV_EMPTY}-after
escaped = $$ENV{SNCLIENT_TEST_ENV_HOST}
append = $ENV{SNCLIENT_TEST_ENV_HOST}
append += -$ENV{SNCLIENT_TEST_ENV_HOST}
ondemand = ${/settings/default/referenced}
`
	cfg := NewConfig(false)
	err := cfg.ParseINI(configText, "testfile.ini", nil)
	require.NoError(t, err)

	allowed, _ := cfg.Section("/settings/default").GetString("allowed hosts")
	assert.Equal(t, "127.0.0.1, 10.1.2.3", allowed)

	password, _ := cfg.Section("/settings/WEB/server").GetString("password")
	assert.Equal(t, "s3cret", password)

	testSection := cfg.Section("/test")
	combined, _ := testSection.GetString("combined")
	assert.Equal(t, "prefix-10.1.2.3-suffix", combined)
	empty, _ := testSection.GetString("empty")
	assert.Equal(t, "before--after", empty)
	escaped, _ := testSection.GetString("escaped")
	assert.Equal(t, "$ENV{SNCLIENT_TEST_ENV_HOST}", escaped)
	appended, _ := testSection.GetString("append")
	assert.Equal(t, "10.1.2.3-10.1.2.3", appended)
	onDemand, _ := testSection.GetString("ondemand")
	assert.Equal(t, "10.1.2.3", onDemand)

	// direct data access also returns the expanded value (check_multi uses it)
	assert.Equal(t, "prefix-10.1.2.3-suffix", testSection.data["combined"])

	// raw and serialized config keep the placeholder
	raw, _, ok := cfg.Section("/settings/WEB/server").GetStringRaw("password")
	require.True(t, ok)
	assert.Equal(t, []string{`"$ENV{SNCLIENT_TEST_ENV_PASSWORD}"`}, raw)
	assert.Contains(t, cfg.ToString(), "$ENV{SNCLIENT_TEST_ENV_PASSWORD}")
	assert.NotContains(t, cfg.ToString(), "s3cret")
}

func TestConfigEnvMacrosMissing(t *testing.T) {
	unsetEnv(t, "SNCLIENT_TEST_ENV_MISSING")

	configText := "[/test]\nkey = $ENV{SNCLIENT_TEST_ENV_MISSING}\n"
	cfg := NewConfig(false)
	err := cfg.ParseINI(configText, "testfile.ini", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "testfile.ini:2")
	assert.Contains(t, err.Error(), `environment variable "SNCLIENT_TEST_ENV_MISSING" is not set`)

	// keep the unexpanded value so a config write does not lose the setting
	val, ok := cfg.Section("/test").GetString("key")
	require.True(t, ok)
	assert.Equal(t, "$ENV{SNCLIENT_TEST_ENV_MISSING}", val)
	assert.Contains(t, cfg.ToString(), "$ENV{SNCLIENT_TEST_ENV_MISSING}")
}

func TestConfigEnvMacrosInclude(t *testing.T) {
	tmpDir := t.TempDir()
	includeFile := filepath.Join(tmpDir, "included.ini")
	require.NoError(t, os.WriteFile(includeFile, []byte("[/included]\nkey = value\n"), 0o600))
	t.Setenv("SNCLIENT_TEST_ENV_INCLUDE", includeFile)

	configText := "[/includes]\nlocal = $ENV{SNCLIENT_TEST_ENV_INCLUDE}\n"
	cfg := NewConfig(true)
	err := cfg.ParseINI(configText, filepath.Join(tmpDir, "main.ini"), nil)
	require.NoError(t, err)

	val, ok := cfg.Section("/included").GetString("key")
	require.True(t, ok)
	assert.Equal(t, "value", val)
}

func TestConfigEnvMacrosIncludeMissing(t *testing.T) {
	unsetEnv(t, "SNCLIENT_TEST_ENV_INCLUDE_MISSING")

	configText := "[/includes]\nlocal = $ENV{SNCLIENT_TEST_ENV_INCLUDE_MISSING}\n"
	cfg := NewConfig(true)
	err := cfg.ParseINI(configText, "main.ini", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `environment variable "SNCLIENT_TEST_ENV_INCLUDE_MISSING" is not set`)
}

func TestConfigEnvMacrosRaw(t *testing.T) {
	t.Setenv("SNCLIENT_TEST_ENV_COMMAND", "echo hello")

	configText := "[/settings/external scripts/scripts/test]\ncommand = $ENV{SNCLIENT_TEST_ENV_COMMAND} $ARG1$\n"
	cfg := NewConfig(false)
	require.NoError(t, cfg.ParseINI(configText, "testfile.ini", nil))
	section := cfg.Section("/settings/external scripts/scripts/test")

	raw, val, ok := section.GetStringRaw("command")
	require.True(t, ok)
	assert.Equal(t, "echo hello $ARG1$", val)
	assert.Equal(t, []string{"$ENV{SNCLIENT_TEST_ENV_COMMAND} $ARG1$"}, raw)

	require.NoError(t, cfg.ReplaceMacrosDefault(section, nil))

	raw, val, ok = section.GetStringRaw("command")
	require.True(t, ok)
	assert.Equal(t, "echo hello $ARG1$", val)
	assert.Equal(t, []string{"echo hello $ARG1$"}, raw)
}

func TestConfigEnvMacrosPaths(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("SNCLIENT_TEST_ENV_SHARED", tmpDir)

	configText := `
[/paths]
shared-path = $ENV{SNCLIENT_TEST_ENV_SHARED}

[/settings/default]
key = $ENV{SNCLIENT_TEST_ENV_SHARED}
`
	iniFile := filepath.Join(t.TempDir(), "snclient.ini")
	require.NoError(t, os.WriteFile(iniFile, []byte(configText), 0o600))

	snc := &Agent{}
	initSet, err := snc.ReadConfiguration([]string{iniFile})
	require.NoError(t, err)

	shared, _ := initSet.config.Section("/paths").GetString("shared-path")
	assert.Equal(t, tmpDir, shared)
	key, _ := initSet.config.Section("/settings/default").GetString("key")
	assert.Equal(t, tmpDir, key)
}
