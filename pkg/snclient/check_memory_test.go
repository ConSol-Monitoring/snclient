package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckMemory(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_memory", []string{"warn=used > 95", "crit=used > 98"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		regexp.MustCompile(`^OK: committed = \d+(\.\d+)? [KMGTi]*B, physical = \d+(\.\d+)? [KMGTi]*B \|`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.Regexpf(t,
		regexp.MustCompile(`'physical'=\d+B;\d+;\d+;0;\d+\s*`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.Regexpf(t,
		regexp.MustCompile(`'physical %'=\d+(\.\d+)?%;\d+(\.\d+)?;\d+(\.\d+)?;0;100\s*`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.Regexpf(t,
		regexp.MustCompile(`'committed'=\d+B;\d+;\d+;0;\d+\s*`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.Regexpf(t,
		regexp.MustCompile(`'committed %'=\d+(\.\d+)?%;\d+(\.\d+)?;\d+(\.\d+)?;0;100\s*`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_memory", []string{"warn=used > 1B", "crit=used > 10B"})
	assert.Equalf(t, CheckExitCritical, res.State, "state CRITICAL")
	assert.Regexpf(t,
		regexp.MustCompile(`^CRITICAL: committed = \d+(\.\d+)? [KMGTi]*B, physical = \d+(\.\d+)? [KMGTi]*B \|`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.Regexpf(t,
		regexp.MustCompile(`'physical'=\d+B;1;10;0;\d+\s*`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_memory", []string{"warn=free_pct < 5", "crit=free_pct < 1"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		regexp.MustCompile(`^OK: committed = \d+(\.\d+)? [KMGTi]*B, physical = \d+(\.\d+)? [KMGTi]*B \|`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.Regexpf(t,
		regexp.MustCompile(`'committed_free_pct'=\d+(\.\d+)?%;\d+(\.\d+)?:;\d+(\.\d+)?:;0;100\s*`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.Regexpf(t,
		regexp.MustCompile(`'physical_free_pct'=\d+(\.\d+)?%;\d+(\.\d+)?:;\d+(\.\d+)?:;0;100\s*`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
	StopTestAgent(t, snc)
}
