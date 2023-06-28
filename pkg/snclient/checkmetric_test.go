package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckMetricsString(t *testing.T) {
	for _, check := range []struct {
		metric CheckMetric
		expect string
	}{
		{CheckMetric{Name: "val", Value: "13", Unit: "B"}, `'val'=13B`},
		{CheckMetric{Name: "val", Value: "0.5", Unit: ""}, `'val'=0.5`},
		{CheckMetric{Name: "val", Value: "U", Unit: ""}, `'val'=U`},
	} {
		res := check.metric.String()
		assert.Equalf(t, check.expect, res, "CheckMetric.String() ->> %s", res)
	}
}
