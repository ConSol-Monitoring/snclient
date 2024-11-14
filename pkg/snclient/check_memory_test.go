package snclient

import (
	"runtime"
	"testing"

	"github.com/shirou/gopsutil/v4/mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckMemory(t *testing.T) {
	snc := StartTestAgent(t, "")

	swap, err := mem.SwapMemory()
	require.NoErrorf(t, err, "acquiring swap info failed")

	swapName := "committed"
	if runtime.GOOS != "windows" {
		swapName = "swap"
	}

	hasSwap := false
	expectedOKOutput := `^OK - physical = \d+(\.\d+)? [KMGTi]*B\/\d+(\.\d+)? [KMGTi]*B \(\d+.\d+%\) \|`
	expectedCriticalOutput := `^CRITICAL - physical = \d+(\.\d+)? [KMGTi]*B\/\d+(\.\d+)? [KMGTi]*B \(\d+.\d+%\) \|`
	if swap.Total > 0 {
		hasSwap = true
		expectedOKOutput = `^OK - physical = \d+(\.\d+)? [KMGTi]*B\/\d+(\.\d+)? [KMGTi]*B \(\d+.\d+%\), ` + swapName + ` = \d+(\.\d+)? [KMGTi]*B\/\d+(\.\d+)? [KMGTi]*B \(\d+.\d+%\) \|`
		expectedCriticalOutput = `^CRITICAL - physical = \d+(\.\d+)? [KMGTi]*B\/\d+(\.\d+)? [KMGTi]*B \(\d+.\d+%\), ` +
			swapName + ` = \d+(\.\d+)? [KMGTi]*B\/\d+(\.\d+)? [KMGTi]*B \(\d+.\d+%\) \|`
	}

	res := snc.RunCheck("check_memory", []string{"warn=used > 101", "crit=used > 102"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		expectedOKOutput,
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.Regexpf(t,
		`'physical'=\d+B;\d+;\d+;0;\d+\s*`,
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.Regexpf(t,
		`'physical %'=\d+(\.\d+)?%;\d+(\.\d+)?;\d+(\.\d+)?;0;100\s*`,
		string(res.BuildPluginOutput()),
		"output matches",
	)
	if hasSwap {
		assert.Regexpf(t,
			`'`+swapName+`'=\d+B;\d+;\d+;0;\d+\s*`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
		assert.Regexpf(t,
			`'`+swapName+` %'=\d+(\.\d+)?%;\d+(\.\d+)?;\d+(\.\d+)?;0;100\s*`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	}

	res = snc.RunCheck("check_memory", []string{"warn=used > 1B", "crit=used > 10B"})
	assert.Equalf(t, CheckExitCritical, res.State, "state CRITICAL")
	assert.Regexpf(t,
		expectedCriticalOutput,
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.Regexpf(t,
		`'physical'=\d+B;1;10;0;\d+\s*`,
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_memory", []string{"warn=free_pct < 0", "crit=free_pct < 0"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		expectedOKOutput,
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.Regexpf(t,
		`'physical_free %'=\d+(\.\d+)?%;\d+(\.\d+)?:;\d+(\.\d+)?:;0;100\s*`,
		string(res.BuildPluginOutput()),
		"output matches",
	)
	if hasSwap {
		assert.Regexpf(t,
			`'`+swapName+`_free %'=\d+(\.\d+)?%;\d+(\.\d+)?:;\d+(\.\d+)?:;0;100\s*`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	}

	res = snc.RunCheck("check_memory", []string{"type=virtual"})
	assert.NotEmptyf(t, res.Output, "got something from virtual")
	if runtime.GOOS == "windows" {
		assert.Equalf(t, CheckExitOK, res.State, "state OK")
		assert.Containsf(t, string(res.BuildPluginOutput()), "OK - virtual", "output matches")
	} else {
		assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN")
		assert.Containsf(t, string(res.BuildPluginOutput()), "UNKNOWN - virtual memory is only supported on windows", "output matches")
	}

	StopTestAgent(t, snc)
}
