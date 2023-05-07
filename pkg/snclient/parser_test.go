package snclient

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArgumentParser(t *testing.T) {
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
		args, err := ParseArgs(check.args, &check.data)
		assert.NoErrorf(t, err, "ParseArgs")
		assert.Equal(t, check.expect, args, fmt.Sprintf("ParseArgs(%s) -> %v", check.args, check.expect))
	}
}

func TestThresholdParser(t *testing.T) {
	for _, check := range []struct {
		threshold string
		expect    *Threshold
	}{
		{"load > 95%", &Threshold{name: "load_pct", operator: ">", value: "95", unit: "%"}},
		{"used > 90GB", &Threshold{name: "used", operator: ">", value: "90000000000", unit: "B"}},
		{"state = dead", &Threshold{name: "state", operator: "=", value: "dead", unit: ""}},
		{"uptime < 180s", &Threshold{name: "uptime", operator: "<", value: "180", unit: "s"}},
		{"version not like  '1 2 3'", &Threshold{name: "version", operator: "not like", value: "1 2 3", unit: ""}},
	} {
		thr, err := ThresholdParse(check.threshold)
		assert.NoErrorf(t, err, "ParseThreshold")
		assert.Equal(t, check.expect, thr, fmt.Sprintf("ParseArgs(%s) -> %v", check.threshold, check.expect))
	}
}
