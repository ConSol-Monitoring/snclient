package snclient

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListenerConfig(t *testing.T) {
	conf := &ConfigSection{
		data: ConfigData{
			"port":          "8080",
			"bind to":       "*",
			"allowed hosts": "localhost, [::1], 127.0.0.1, 192.168.123.0/24, one.one.one.one",
			"use ssl":       "false",
		},
	}

	listen := Listener{}
	err := listen.setListenConfig(conf)
	require.NoErrorf(t, err, "setListenConfig should not return an error")

	ahc, err := NewAllowedHostConfig(conf)
	require.NoErrorf(t, err, "allowed host config parsed")

	for _, check := range []struct {
		expect bool
		addr   string
	}{
		{true, "127.0.0.1"},
		{false, "127.0.0.2"},
		{true, "192.168.123.1"},
		{false, "192.168.125.5"},
		{true, "1.1.1.1"},
	} {
		assert.Equalf(t, check.expect, ahc.Check(check.addr), "CheckAllowedHosts(%s) -> %v", check.addr, check.expect)
	}
}

func TestListenerSharedPort(t *testing.T) {
	config := `
	[/modules]
	WEBServer = enabled
	ExporterExporterServer = enabled
	PrometheusServer = enabled

	[/settings/WEB/server]
	port = 45666
	use ssl = false

	[/settings/Prometheus/server]
	port = 45666
	use ssl = false

	[/settings/ExporterExporter/server]
	port = ${/settings/WEB/server/port}
	use ssl = ${/settings/WEB/server/use ssl}
	`
	snc := StartTestAgent(t, config)

	_, err := net.DialTimeout("tcp", "127.0.0.1:45666", 10*time.Second)
	require.NoErrorf(t, err, "connection established")

	StopTestAgent(t, snc)
}
