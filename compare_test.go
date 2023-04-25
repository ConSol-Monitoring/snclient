package snclient

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareMetrics(t *testing.T) {
	t.Parallel()

	for _, check := range []struct {
		metrics  map[string]string
		treshold Treshold
		expect   bool
	}{
		{
			map[string]string{"used": "13958643712"},
			Treshold{name: "used", operator: ">", value: "13", unit: "GB"},
			true,
		},
		{
			map[string]string{"used_pct": "13"},
			Treshold{name: "used_pct", operator: ">", value: "80", unit: "%"},
			false,
		},
		{
			map[string]string{"status": "4"},
			Treshold{name: "status", operator: "!=", value: "4", unit: ""},
			false,
		},
		{
			map[string]string{"uptime": "428173"},
			Treshold{name: "uptime", operator: "<", value: "180", unit: "s"},
			false,
		},
	} {
		assert.Equal(t, check.expect, CompareMetrics(check.metrics, check.treshold),
			fmt.Sprintf("CompareMetrics(%v, %v) -> %v", check.metrics, check.treshold, check.expect))
	}
}
