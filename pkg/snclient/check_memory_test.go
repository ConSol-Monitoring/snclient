package snclient

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/shirou/gopsutil/v4/mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckMemoryUseFree(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("uses Linux meminfo")
	}
	snc := StartTestAgent(t, "")
	defer StopTestAgent(t, snc)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "meminfo"), []byte("MemTotal: 1000 kB\nMemFree: 150 kB\nMemAvailable: 500 kB\nCached: 300 kB\n"), 0o600))
	t.Setenv("HOST_PROC", dir)
	args := []string{
		"type=physical", "detail-syntax=%(used_pct)",
		"perf-config=size_bytes(unit:KB;magic:0.001;suffix:TOTAL) used_bytes(unit:KB;magic:0.001;suffix:USED) " +
			"free_bytes(unit:KB;magic:0.001;suffix:FREE) caches_bytes(unit:KB;magic:0.001;suffix:CACHES) *(ignore:yes)",
	}
	res := snc.RunCheck("check_memory", args)
	assert.Equal(t, CheckExitOK, res.State)
	assert.Contains(t, string(res.BuildPluginOutput()), "'USED'=512KB")

	res = snc.RunCheck("check_memory", append([]string{"--use-free"}, args...))
	assert.Equal(t, CheckExitWarning, res.State)
	output := string(res.BuildPluginOutput())
	assert.Contains(t, output, "WARNING - 85.000")
	assert.Contains(t, output, "'TOTAL'=1024KB")
	assert.Contains(t, output, "'USED'=870.4KB")
	assert.Contains(t, output, "'FREE'=153.6KB")
	assert.Contains(t, output, "'CACHES'=307.2KB")
}

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
	assert.Regexpf(
		t,
		expectedOKOutput,
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.Regexpf(
		t,
		`'physical'=\d+B;\d+;\d+;0;\d+\s*`,
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.Regexpf(
		t,
		`'physical %'=\d+(\.\d+)?%;\d+(\.\d+)?;\d+(\.\d+)?;0;100\s*`,
		string(res.BuildPluginOutput()),
		"output matches",
	)
	if hasSwap {
		assert.Regexpf(
			t,
			`'`+swapName+`'=\d+B;\d+;\d+;0;\d+\s*`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
		assert.Regexpf(
			t,
			`'`+swapName+` %'=\d+(\.\d+)?%;\d+(\.\d+)?;\d+(\.\d+)?;0;100\s*`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	}

	res = snc.RunCheck("check_memory", []string{"warn=used > 1B", "crit=used > 10B"})
	assert.Equalf(t, CheckExitCritical, res.State, "state CRITICAL")
	assert.Regexpf(
		t,
		expectedCriticalOutput,
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.Regexpf(
		t,
		`'physical'=\d+B;1;10;0;\d+\s*`,
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_memory", []string{"warn=free_pct < 0", "crit=free_pct < 0"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(
		t,
		expectedOKOutput,
		string(res.BuildPluginOutput()),
		"output matches",
	)
	assert.Regexpf(
		t,
		`'physical_free %'=\d+(\.\d+)?%;\d+(\.\d+)?:;\d+(\.\d+)?:;0;100\s*`,
		string(res.BuildPluginOutput()),
		"output matches",
	)
	if hasSwap {
		assert.Regexpf(
			t,
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

	res = snc.RunCheck("check_memory", []string{"type=physical", "detail-syntax=%(type) = %(used)/%(size) free: %(free) caches: %(caches) (%(caches_pct | fmt=%.1f )%)", "warn=none", "crit=none"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	memoryValue := `\d+(\.\d+)? [KMGTi]*B`
	assert.Regexpf(t, `^OK - physical = `+memoryValue+`/`+memoryValue+` free: `+memoryValue+` caches: `+memoryValue+` \(\d+\.\d%\)$`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_memory", []string{"type=physical", "warn=caches > 101", "crit=caches > 102"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t, `'physical_caches'=\d+B;`, string(res.BuildPluginOutput()), "output matches")
	assert.Regexpf(t, `'physical_caches %'=\d+(\.\d+)?%;`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_memory", []string{"type=physical", "warn=none", "crit=none", "perf-config=caches(unit:K)"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t, `'physical_caches'=[\d.]+K;`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_memory", []string{"type=physical", "warn=none", "crit=none", "perf-config=*(unit:K)"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.NotContains(t, string(res.BuildPluginOutput()), "physical_caches", "bare wildcard must not enable metrics")

	res = snc.RunCheck("check_memory", []string{"type=physical", "warn=none", "crit=none", "-n", "10", "--show-args"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "RSS", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "%MEM", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "COMMAND", "output matches")

	StopTestAgent(t, snc)
}
