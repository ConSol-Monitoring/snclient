package snclient

import (
	"fmt"
	"os"
	"testing"

	_ "pkg/dump"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	assert.Truef(t, snc.verifyPassword(p0, "test"), "password check disabled -> ok")
	assert.Truef(t, snc.verifyPassword(p0, ""), "password check disabled -> ok")

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
	tmpInclude, err := os.CreateTemp("", "testconfig")
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
