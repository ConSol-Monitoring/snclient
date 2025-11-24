//go:build linux
// +build linux

package snclient

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func checkFilesConfigfile(t *testing.T, scriptsDir, scriptsType string) string {
	t.Helper()

	config := fmt.Sprintf(`
[/modules]
CheckExternalScripts = enabled
[/paths]
scripts = %s
shared-path = %%(scripts)
[/settings/external scripts/wrappings]
sh = %%SCRIPT%% %%ARGS%%
exe = %%SCRIPT%% %%ARGS%%
[/settings/external scripts]
timeout = 1111111
allow arguments = true
[/settings/external scripts/scripts]
check_files_generate_files = ./check_files_generate_files.EXTENSION "$ARG1$"
`, scriptsDir)

	config = strings.ReplaceAll(config, "EXTENSION", scriptsType)

	return config
}

func TestTimeKeywordFilters(t *testing.T) {
	// prepare a tempdir
	tempDir := t.TempDir()

	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := checkFilesConfigfile(t, scriptsDir, "sh")
	snc := StartTestAgent(t, config)

	// There is a bash script on this path: pkg/snclient/t/scripts/check_files_generate_files.sh
	// It generates files on a temporary path, and changes their modification date
	// This script is added to the snclient config first and registered as a check command, then ran by the snclient executable itself
	res := snc.RunCheck("check_files_generate_files", []string{tempDir})

	// The script generates 11 files:
	// one_year_from_now_on
	// one_month_from_now_on
	// one_week_from_now_on
	// two_days_from_now_on
	// tomorrow
	// today
	// yesterday
	// two_days_ago
	// one_week_ago
	// one_month_ago
	// one_year_ago
	assert.Equalf(t, CheckExitOK, res.State, "Generating test files successful")
	assert.Equalf(t, "ok - Generated 11 files for testing", string(res.BuildPluginOutput()), "output matches")

	// This will be printed if the test fails.
	t.Logf("Contents of test directory %s:", tempDir)
	files, _ := os.ReadDir(tempDir)
	for _, file := range files {
		info, _ := file.Info()
		t.Logf("- File: %s, ModTime: %s", file.Name(), info.ModTime().Format(time.RFC3339))
	}

	// Note on 2025-11-06 : Multiple filter="<condition>"s are combined with a logical OR.
	// res = snc.RunCheck("check_files", []string{fmt.Sprintf("path=%s", tempDir), "filter=\"written>=today\"", "filter=\"written<tomorrow\""})
	// Such a test got every file

	// combine the two conditions, filters only to the single 'today' file that is written after today midnight and earlier then tomorrow midnight
	res = snc.RunCheck("check_files", []string{fmt.Sprintf("path=%s", tempDir), "filter=\"written>=today && written<tomorrow\""})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 1 files are ok", "output matches")

	// Should get these five files, it cant get today because for that written==today
	// tomorrow
	// two_days_from_now_on
	// one_week_from_now_on
	// one_month_from_now_on
	// one_year_from_now_on
	res = snc.RunCheck("check_files", []string{fmt.Sprintf("path=%s", tempDir), "filter=\"written>today\""})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 5 files are ok", "output matches")

	StopTestAgent(t, snc)
}
