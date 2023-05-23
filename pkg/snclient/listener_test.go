package snclient

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.NoErrorf(t, err, "setListenConfig should not return an error")

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
		assert.Equalf(t, check.expect, listen.CheckAllowedHosts(check.addr), fmt.Sprintf("CheckAllowedHosts(%s) -> %v", check.addr, check.expect))
	}
}
