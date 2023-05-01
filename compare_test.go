package snclient

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareMetrics(t *testing.T) {
	t.Parallel()

	for _, check := range []struct {
		metrics   map[string]string
		threshold Threshold
		expect    bool
	}{
		{
			map[string]string{"used": "13958643712"},
			Threshold{name: "used", operator: ">", value: "13", unit: "GB"},
			true,
		},
		{
			map[string]string{"used_pct": "13"},
			Threshold{name: "used_pct", operator: ">", value: "80", unit: "%"},
			false,
		},
		{
			map[string]string{"status": "4"},
			Threshold{name: "status", operator: "!=", value: "4", unit: ""},
			false,
		},
		{
			map[string]string{"uptime": "428173"},
			Threshold{name: "uptime", operator: "<", value: "180", unit: "s"},
			false,
		},
	} {
		assert.Equal(t, check.expect, CompareMetrics(check.metrics, check.threshold),
			fmt.Sprintf("CompareMetrics(%v, %v) -> %v", check.metrics, check.threshold, check.expect))
	}
}
