package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckPerfConfigParser(t *testing.T) {
	perf, err := NewPerfConfig("used(unit:G;suffix:'s'; prefix:'pre') used %(ignored:true) *(unit:GiB)  ")
	require.NoErrorf(t, err, "no error in NewPerfConfig")

	exp := []PerfConfig{
		{Selector: "used", Unit: "G", Suffix: "s", Prefix: "pre", Magic: 1, Raw: `used(unit:G;suffix:'s'; prefix:'pre')`},
		{Selector: "used %", Ignore: true, Magic: 1, Raw: `used %(ignored:true)`},
		{Selector: "*", Unit: "GiB", regex: regexp.MustCompile(".*"), Magic: 1, Raw: `*(unit:GiB)`},
	}
	assert.Equalf(t, exp, perf, "NewPerfConfig parsed correctly")
}

func TestCheckPerfConfig(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_memory", []string{
		"type=physical",
		"warn=used > 101",
		"crit=used > 102",
		"perf-config=physical %(ignored:true) *(unit:MB;prefix:gib_;suffix:phy)",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		regexp.MustCompile(`^OK - physical = [\d.]+ \w+\/[\d.]+ \w+ \([\d.]+%\) \|'gib_phy'=[\d.]+MB;[\d.]+;[\d.]+;0;[\d.]+$`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.NotContains(t, string(res.BuildPluginOutput()), "physical %")

	res = snc.RunCheck("check_memory", []string{
		"type=physical",
		"warn=used > 1",
		"warn=used < 100",
		"perf-config=physical %(ignored:true) *(unit:MB;prefix:gib_;suffix:phy)",
	})
	assert.Equalf(t, CheckExitWarning, res.State, "state WARNING")
	assert.Regexpf(t,
		regexp.MustCompile(`^WARNING - physical = [\d.]+ \w+\/[\d.]+ \w+ \([\d.]+%\) \|'gib_phy'=[\d.]+MB;@[\d.]+:[\d.]+;[\d.]+;0;[\d.]+$`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.NotContains(t, string(res.BuildPluginOutput()), "physical %")

	res = snc.RunCheck("check_memory", []string{
		"type=physical",
		"warn=used > 1",
		"warn=used < 100",
		"perf-config=*(unit:mib)",
	})
	assert.Equalf(t, CheckExitWarning, res.State, "state WARNING")
	assert.Regexpf(t,
		regexp.MustCompile(`^WARNING - physical = [\d.]+ \w+\/[\d.]+ \w+ \([\d.]+%\) \|'physical'=[\d.]+mib;@[\d.]+:[\d.]+;[\d.]+;0;[\d.]+`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.NotContainsf(t, string(res.BuildPluginOutput()), "=0mib;", "must not contain 0mib")
	// perf config should not break printing thresholds
	assert.NotContainsf(t, string(res.BuildPluginOutput()), ";@0:0;", "must not contain ;@0:0;")

	res = snc.RunCheck("check_uptime", []string{
		"warn=uptime > 0",
		"crit=uptime > 0",
		"perf-config=*(unit:d)",
	})
	assert.Equalf(t, CheckExitCritical, res.State, "state Critical")
	assert.Regexpf(t,
		regexp.MustCompile(`'uptime'=[\d.]+d;0;0`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	StopTestAgent(t, snc)
}

func TestCheckPerfSyntax(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_memory", []string{
		"type=physical",
		"warn=used > 101",
		"crit=used > 102",
		"perf-syntax='mem:%(key | uc)'",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		regexp.MustCompile(`'mem:PHYSICAL %'=[\d.]+%;101;102;0;100`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	StopTestAgent(t, snc)
}
