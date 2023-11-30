package snclient

import (
	"regexp"
	"testing"

	"github.com/shirou/gopsutil/v3/mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckMemory(t *testing.T) {
	snc := StartTestAgent(t, "")

	swap, err := mem.SwapMemory()
	require.NoErrorf(t, err, "acquiring swap info failed")

	hasSwap := false
	expectedOKOutput := `^OK: physical = \d+(\.\d+)? [KMGTi]*B\/\d+(\.\d+)? [KMGTi]*B \(\d+.\d+%\) \|`
	expectedCriticalOutput := `^CRITICAL: physical = \d+(\.\d+)? [KMGTi]*B\/\d+(\.\d+)? [KMGTi]*B \(\d+.\d+%\) \|`
	if swap.Total > 0 {
		hasSwap = true
		expectedOKOutput = `^OK: physical = \d+(\.\d+)? [KMGTi]*B\/\d+(\.\d+)? [KMGTi]*B \(\d+.\d+%\), committed = \d+(\.\d+)? [KMGTi]*B\/\d+(\.\d+)? [KMGTi]*B \(\d+.\d+%\) \|`
		expectedCriticalOutput = `^CRITICAL: physical = \d+(\.\d+)? [KMGTi]*B\/\d+(\.\d+)? [KMGTi]*B \(\d+.\d+%\), committed = \d+(\.\d+)? [KMGTi]*B\/\d+(\.\d+)? [KMGTi]*B \(\d+.\d+%\) \|`
	}

	res := snc.RunCheck("check_memory", []string{"warn=used > 101", "crit=used > 102"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		regexp.MustCompile(expectedOKOutput),
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
	if hasSwap {
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
	}

	res = snc.RunCheck("check_memory", []string{"warn=used > 1B", "crit=used > 10B"})
	assert.Equalf(t, CheckExitCritical, res.State, "state CRITICAL")
	assert.Regexpf(t,
		regexp.MustCompile(expectedCriticalOutput),
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.Regexpf(t,
		regexp.MustCompile(`'physical'=\d+B;1;10;0;\d+\s*`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_memory", []string{"warn=free_pct < 0", "crit=free_pct < 0"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		regexp.MustCompile(expectedOKOutput),
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.Regexpf(t,
		regexp.MustCompile(`'physical_free %'=\d+(\.\d+)?%;\d+(\.\d+)?:;\d+(\.\d+)?:;0;100\s*`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
	if hasSwap {
		assert.Regexpf(t,
			regexp.MustCompile(`'committed_free %'=\d+(\.\d+)?%;\d+(\.\d+)?:;\d+(\.\d+)?:;0;100\s*`),
			string(res.BuildPluginOutput()),
			"output matches",
		)
	}
	StopTestAgent(t, snc)
}
