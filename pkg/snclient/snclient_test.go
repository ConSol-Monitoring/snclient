package snclient

import (
	"fmt"
	"os"
	"testing"

	_ "github.com/consol-monitoring/snclient/pkg/dump"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Generates a config file, where snclient can call a script.
// scriptName does not have an extension
// scriptFilename does have (most likely an OS specific) script extension.
// It registers four commands for script
// scriptName_arg1 : ./${SCRIPT_FILENAME} "$ARG1$"
// scriptName_arg_numbered : ./${SCRIPT_FILENAME} "$ARG1$" "$ARG2$" "$ARG3$" "$ARG4$" "$ARG5$" "$ARG6$" "$ARG7$" "$ARG8$" "$ARG9$" "$ARG10$"
// scriptName_args : ./${SCRIPT_FILENAME} "$ARGS$"
// scriptName_args_quouted : ./${SCRIPT_FILENAME} "$ARGS"$"
func snclientConfigFileWithScript(t *testing.T, scriptsDir, scriptName, scriptFilename string) string {
	t.Helper()

	configTemplate := `
[/modules]
CheckExternalScripts = enabled

[/paths]
scripts = ${SCRIPTS_DIR}
shared-path = %(scripts)

[/settings/external scripts]
timeout = 1111111
allow arguments = true

[/settings/external scripts/scripts]
${SCRIPT_NAME}_arg1 = ./${SCRIPT_FILENAME} $ARG1$

[/settings/external scripts/scripts/${SCRIPT_NAME}_arg1]
allow arguments = true
allow nasty characters = true

[/settings/external scripts/scripts]
${SCRIPT_NAME}_arg_numbered = ./${SCRIPT_FILENAME} $ARG1$ $ARG2$ $ARG3$ $ARG4$ $ARG5$ $ARG6$ $ARG7$ $ARG8$ $ARG9$ $ARG10$

[/settings/external scripts/scripts/${SCRIPT_NAME}_arg_numbered]
allow arguments = true
allow nasty characters = true

[/settings/external scripts/scripts]
${SCRIPT_NAME}_args = ./${SCRIPT_FILENAME} $ARGS$

[/settings/external scripts/scripts/${SCRIPT_NAME}_args]
allow arguments = true
allow nasty characters = true

[/settings/external scripts/scripts]
${SCRIPT_NAME}_args_quouted = ./${SCRIPT_FILENAME} $ARGS"$

[/settings/external scripts/scripts/${SCRIPT_NAME}_args_quouted]
allow arguments = true
allow nasty characters = true
`

	mapper := func(placeholderName string) string {
		switch placeholderName {
		case "SCRIPTS_DIR":
			return scriptsDir
		case "SCRIPT_NAME":
			return scriptName
		case "SCRIPT_FILENAME":
			return scriptFilename
		default:
			// if its not some value we know, leave it as is
			return "$" + placeholderName
		}
	}

	return os.Expand(configTemplate, mapper)
}

func TestPasswords(t *testing.T) {
	config := fmt.Sprintf(`
[/settings]
password0 =
password1 = %s
password2 = secret
password3 = SHA256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08
`, DefaultPassword)

	snc := StartTestAgent(t, config)
	conf := snc.config.Section("/settings")

	disableLogsTemporarily()
	defer restoreLogLevel()

	p0, _ := conf.GetString("password0")
	assert.Falsef(t, snc.verifyPassword(p0, "test"), "password check disabled for empty passwords -> not ok")
	assert.Falsef(t, snc.verifyPassword(p0, ""), "password check disabled -> not ok")

	p1, _ := conf.GetString("password1")
	assert.Falsef(t, snc.verifyPassword(p1, "test"), "default password still set -> not ok")
	assert.Falsef(t, snc.verifyPassword(p1, DefaultPassword), "default password still set -> not ok")

	p2, _ := conf.GetString("password2")
	assert.Truef(t, snc.verifyPassword(p2, "secret"), "simple password -> ok")
	assert.Falsef(t, snc.verifyPassword(p2, "wrong"), "simple password wrong")

	p3, _ := conf.GetString("password3")
	assert.Truef(t, snc.verifyPassword(p3, "test"), "hashed password -> ok")
	assert.Falsef(t, snc.verifyPassword(p3, "wrong"), "hashed password wrong")

	StopTestAgent(t, snc)
}

func TestConfigInheritance(t *testing.T) {
	tmpInclude, err := os.CreateTemp(t.TempDir(), "testconfig")
	require.NoErrorf(t, err, "tmp config created")

	config := fmt.Sprintf(`
[/modules]
WEBServer = enabled
CheckExternalScripts = enabled

[/settings/default]
allowed hosts = 127.0.0.1, ::1

password = CHANGEME

[/includes]
local = %s
`, tmpInclude.Name())

	_, err = tmpInclude.WriteString(`
[/settings/default]
allowed hosts = ::1, 127.0.0.1, 123.123.123.123

[/settings/WEB/server]
port = 45666
use ssl = false
password = test
`)
	require.NoErrorf(t, err, "tmp include created")
	err = tmpInclude.Close()
	require.NoErrorf(t, err, "tmp include created")

	snc := StartTestAgent(t, config)

	allowed, ok := snc.config.Section("/settings/default").GetString("allowed hosts")
	assert.True(t, ok)
	assert.Contains(t, allowed, "123.123.123.123")

	allowed, ok = snc.config.Section("/settings/WEB/server").GetString("allowed hosts")
	assert.True(t, ok)
	assert.Contains(t, allowed, "123.123.123.123")

	pass, ok := snc.config.Section("/settings/WEB/server").GetString("password")
	assert.True(t, ok)
	assert.Equal(t, "test", pass)

	cmd, ok := snc.config.Section("/settings/external scripts/wrappings").GetString("ps1")
	assert.True(t, ok)
	assert.NotContains(t, cmd, "script root")
	assert.Contains(t, cmd, "%SCRIPT%")

	StopTestAgent(t, snc)
}
