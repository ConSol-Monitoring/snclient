package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListenWebPerfInt(t *testing.T) {
	metric := CheckMetric{
		Name:  "test",
		Unit:  "B",
		Value: 1024,
	}

	l := &HandlerWeb{}
	perf := l.metric2Perf(&metric)

	assert.Equalf(t, "test", perf.Alias, "alias")

	expect := &CheckWebPerfVal{
		Value: CheckWebPerfNumber("1024"),
		Unit:  "B",
	}
	assert.Nilf(t, perf.FloatVal, "float val is empty")
	assert.Equalf(t, perf.IntVal, expect, "float value")
}

func TestListenWebPerfFloat(t *testing.T) {
	metric := CheckMetric{
		Name:  "temp",
		Unit:  "°C",
		Value: 10.5,
	}

	l := &HandlerWeb{}
	perf := l.metric2Perf(&metric)

	assert.Equalf(t, "temp", perf.Alias, "alias")

	expect := &CheckWebPerfVal{
		Value: CheckWebPerfNumber("10.5"),
		Unit:  "°C",
		Min:   nil,
		Max:   nil,
	}
	assert.Nilf(t, perf.IntVal, "int value is empty")
	assert.Equalf(t, perf.FloatVal, expect, "float value")
}

func TestListenWebPerfUnknown(t *testing.T) {
	metric := CheckMetric{
		Name:  "rss",
		Value: "U",
	}

	l := &HandlerWeb{}
	perf := l.metric2Perf(&metric)

	assert.Equalf(t, "rss", perf.Alias, "alias")

	expect := &CheckWebPerfVal{
		Value: "U",
		Unit:  "",
		Min:   nil,
		Max:   nil,
	}
	assert.Nilf(t, perf.FloatVal, "float value is empty")
	assert.Equalf(t, perf.IntVal, expect, "int value")
}
