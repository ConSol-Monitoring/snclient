package snclient

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareMetrics(t *testing.T) {
	for _, check := range []struct {
		metrics  []MetricData
		treshold Treshold
		expect   bool
	}{
		{
			[]MetricData{{name: "used", value: "13958643712"}},
			Treshold{name: "used", operator: ">", value: "13", unit: "GB"},
			true,
		},
		{
			[]MetricData{{name: "used_pct", value: "13"}},
			Treshold{name: "used_pct", operator: ">", value: "80", unit: "%"},
			false,
		},
		{
			[]MetricData{{name: "status", value: "4"}},
			Treshold{name: "status", operator: "!=", value: "4", unit: ""},
			false,
		},
		{
			[]MetricData{{name: "uptime", value: "428173"}},
			Treshold{name: "uptime", operator: "<", value: "180", unit: "s"},
			false,
		},
	} {
		assert.Equal(t, check.expect, CompareMetrics(check.metrics, check.treshold), fmt.Sprintf("CompareMetrics(%v, %v) -> %v", check.metrics, check.treshold, check.expect))
	}
}
