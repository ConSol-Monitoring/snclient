package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckPerfConfigParser(t *testing.T) {
	perf, err := NewPerfConfig("used(unit:G;suffix:'s'; prefix:'pre') used %(ignored:true) *(unit:GiB)  ")
	assert.NoErrorf(t, err, "no error in NewPerfConfig")

	exp := []PerfConfig{
		{Selector: "used", Unit: "G", Suffix: "s", Prefix: "pre", Magic: 1},
		{Selector: "used %", Ignore: true, Magic: 1},
		{Selector: "*", Unit: "GiB", regex: regexp.MustCompile(".*"), Magic: 1},
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
		regexp.MustCompile(`^OK: physical = [\d.]+ \w+ \|'gib_phy'=[\d.]+MB;[\d.]+;[\d.]+;0;[\d.]+$`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.NotContainsf(t, string(res.BuildPluginOutput()), "physical %", "must not contain %")

	res = snc.RunCheck("check_memory", []string{
		"type=physical",
		"warn=used > 1",
		"warn=used < 100",
		"perf-config=physical %(ignored:true) *(unit:MB;prefix:gib_;suffix:phy)",
	})
	assert.Equalf(t, CheckExitWarning, res.State, "state WARNING")
	assert.Regexpf(t,
		regexp.MustCompile(`^WARNING: physical = [\d.]+ \w+ \|'gib_phy'=[\d.]+MB;@[\d.]+:[\d.]+;[\d.]+;0;[\d.]+$`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.NotContainsf(t, string(res.BuildPluginOutput()), "physical %", "must not contain %")

	res = snc.RunCheck("check_memory", []string{
		"type=physical",
		"warn=used > 1",
		"warn=used < 100",
		"perf-config=*(unit:mib)",
	})
	assert.Equalf(t, CheckExitWarning, res.State, "state WARNING")
	assert.Regexpf(t,
		regexp.MustCompile(`^WARNING: physical = [\d.]+ \w+ \|'physical'=[\d.]+mib;@[\d.]+:[\d.]+;[\d.]+;0;[\d.]+`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.NotContainsf(t, string(res.BuildPluginOutput()), "=0mib;", "must not contain 0mib")

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
