package snclient

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArgumentParser(t *testing.T) {
	t.Parallel()

	for _, check := range []struct {
		args   []string
		data   CheckData
		expect []Argument
	}{
		{
			[]string{"service=Dhcp", "warn=load > 95%", "crit=load > 98%"},
			CheckData{},
			[]Argument{
				{key: "service", value: "Dhcp"},
			},
		},
	} {
		assert.Equal(t, check.expect, ParseArgs(check.args, &check.data), fmt.Sprintf("ParseArgs(%s) -> %v", check.args, check.expect))
	}
}

func TestThresholdParser(t *testing.T) {
	t.Parallel()

	for _, check := range []struct {
		threshold string
		expect    *Threshold
	}{
		{"load > 95%", &Threshold{name: "load_pct", operator: ">", value: "95", unit: "%"}},
		{"used > 90GB", &Threshold{name: "used", operator: ">", value: "90", unit: "GB"}},
		{"state = dead", &Threshold{name: "state", operator: "=", value: "dead", unit: ""}},
		{"uptime < 180s", &Threshold{name: "uptime", operator: "<", value: "180", unit: "s"}},
	} {
		assert.Equal(t, check.expect, ParseThreshold(check.threshold), fmt.Sprintf("ParseArgs(%s) -> %v", check.threshold, check.expect))
	}
}
