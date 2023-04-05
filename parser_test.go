package snclient

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArgumentParser(t *testing.T) {
	for _, check := range []struct {
		args   []string
		expect []Argument
	}{
		{
			[]string{"service=Dhcp", "warn=load > 95%", "crit=load > 98%"},
			[]Argument{
				{key: "service", value: "Dhcp"},
				{key: "warn", value: "load > 95%"},
				{key: "crit", value: "load > 98%"},
			},
		},
	} {
		assert.Equal(t, check.expect, ParseArgs(check.args), fmt.Sprintf("ParseArgs(%s) -> %v", check.args, check.expect))
	}
}

func TestTresholdParser(t *testing.T) {
	for _, check := range []struct {
		treshold string
		expect   Treshold
	}{
		{"load > 95%", Treshold{name: "load_pct", operator: ">", value: "95", unit: "%"}},
		{"used > 90GB", Treshold{name: "used", operator: ">", value: "90", unit: "GB"}},
		{"state = dead", Treshold{name: "state", operator: "=", value: "dead", unit: ""}},
		{"uptime < 180s", Treshold{name: "uptime", operator: "<", value: "180", unit: "s"}},
	} {
		assert.Equal(t, check.expect, ParseTreshold(check.treshold), fmt.Sprintf("ParseArgs(%s) -> %v", check.treshold, check.expect))
	}
}
