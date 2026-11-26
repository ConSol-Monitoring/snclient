package snclient

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testLogfileConfig = `
[/modules]
CheckLogFile = enabled

[/settings/check/logfile]
allowed pattern  = **
`

func testLogfileAllowedPatternConfig(allowedPattern string) string {
	return fmt.Sprintf(`
[/modules]
CheckLogFile = enabled

[/settings/check/logfile]
allowed pattern = "%s"
`, allowedPattern)
}

func TestCheckLogFileDisabled(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_logfile", []string{"files=./t/test*"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - module CheckLogFile is not enabled")

	StopTestAgent(t, snc)
}

func TestCheckLogFileNoArguments(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)
	res := snc.RunCheck("check_logfile", []string{})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - no file specified")

	StopTestAgent(t, snc)
}

func TestCheckLogFile(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)
	res := snc.RunCheck("check_logfile", []string{"file=./t/test.log"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK")

	res = snc.RunCheck("check_logfile", []string{"files=./t/test*"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "16")

	StopTestAgent(t, snc)
}

func TestCheckLogFilePathWildCards(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)

	res := snc.RunCheck("check_logfile", []string{"files=./t/test*", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK")

	snc.alreadyParsedLogfiles.Clear()

	res = snc.RunCheck("check_logfile", []string{"files=./t/test*", "warn=line LIKE WARNING"})
	assert.Equalf(t, CheckExitWarning, res.State, "state WARNING")
	assert.Contains(t, string(res.BuildPluginOutput()), "WARNING - 2/16")

	res = snc.RunCheck("check_logfile", []string{"files=./t/test*", "warn=line LIKE WARNING"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - No matching lines found")

	StopTestAgent(t, snc)
}

func TestCheckLogFilePathWildCardsAndOffset0(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)

	res := snc.RunCheck("check_logfile", []string{"files=./t/test*", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK")

	snc.alreadyParsedLogfiles.Clear()

	res = snc.RunCheck("check_logfile", []string{"files=./t/test*", "offset=0", "warn=line LIKE WARNING"})
	assert.Equalf(t, CheckExitWarning, res.State, "state WARNING")
	assert.Contains(t, string(res.BuildPluginOutput()), "WARNING - 2/16")

	res = snc.RunCheck("check_logfile", []string{"files=./t/test*", "offset=0", "warn=line LIKE WARNING"})
	assert.Equalf(t, CheckExitWarning, res.State, "state WARNING")
	assert.Contains(t, string(res.BuildPluginOutput()), "WARNING - 2/16")

	StopTestAgent(t, snc)
}

func TestCheckLogFileOKPatternResetsErrors(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)

	res := snc.RunCheck("check_logfile", []string{"files=./t/test*", "warn=line LIKE ERROR", "ok=line LIKE 'System check completed successfully'"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "16")

	StopTestAgent(t, snc)
}

func TestCheckLogFileFilter(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)

	res := snc.RunCheck("check_logfile", []string{"files=./t/test*", "filter=line LIKE 'WARNING'", "warn=count>1"})
	assert.Equalf(t, CheckExitWarning, res.State, "state WARNING")
	assert.Contains(t, string(res.BuildPluginOutput()), "WARNING - 0/2 ")

	StopTestAgent(t, snc)
}

func TestCheckLogfileLabel(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)

	res := snc.RunCheck("check_logfile", []string{"files=./t/test*", "label='YEAR:^\\d{4}'", "label='ERWAR:(ERROR|WARN)'", "show-all", "detail-syntax=$(ERWAR)$(YEAR)- $(line:cut=50)"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - 2023")

	StopTestAgent(t, snc)
}

func TestCheckLogFileColumnN(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)

	res := snc.RunCheck("check_logfile", []string{"files=./t/test*", "crit=column1 LIKE DEBUG", "column-split=;", "show-all"})
	assert.Equalf(t, CheckExitCritical, res.State, "state CRITICAL")
	assert.Contains(t, string(res.BuildPluginOutput()), "CRITICAL - ")

	StopTestAgent(t, snc)
}

func TestCheckLogFileFileExistsButHasNoLines(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)

	res := snc.RunCheck("check_logfile", []string{"files=./t/test*", "filter=line LIKE this-pattern-does-not-exist-in-the-test-files", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state should be OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - No matching lines")

	StopTestAgent(t, snc)
}

func TestCheckLogFileFileDoesNotExist(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)

	res := snc.RunCheck("check_logfile", []string{"files=./t/testfiledoesnotexist*"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state should be UNKNOWN")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - no files found for search pattern: './t/testfiledoesnotexist*'")

	StopTestAgent(t, snc)
}

func TestCheckLogFileFileDoesNotExist2(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)

	res := snc.RunCheck("check_logfile", []string{"file=./t/testfiledoesnotexist1", "files=./t/testfiledoesnotexist2,./t/testfiledoesnotexist3"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state should be UNKNOWN")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - no files found for search pattern: './t/testfiledoesnotexist1'")

	StopTestAgent(t, snc)
}

func TestCheckLogFileLineExistsOnTheFirstFile(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)

	res := snc.RunCheck("check_logfile", []string{"files=./t/test*", "filter='line LIKE testUser2'"})
	assert.Equalf(t, CheckExitOK, res.State, "state should be OK")

	StopTestAgent(t, snc)
}

func TestCheckLogFileLineSplit(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)

	res := snc.RunCheck("check_logfile", []string{"files=./t/test.log", "offset=0"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - 8 line(s) found")

	res2 := snc.RunCheck("check_logfile", []string{"files=./t/test.log", "offset=0", "line-split='\\n'"})
	assert.Equalf(t, CheckExitOK, res2.State, "state OK")
	assert.Contains(t, string(res2.BuildPluginOutput()), "OK - 8 line(s) found")

	res3 := snc.RunCheck("check_logfile", []string{"files=./t/test.log", "offset=0", "line-split='\\r\\n'"})
	assert.Equalf(t, CheckExitOK, res3.State, "state OK")
	assert.Contains(t, string(res3.BuildPluginOutput()), "OK - 8 line(s) found")

	res3 = snc.RunCheck("check_logfile", []string{"files=./t/test.log", "line-split='\\r\\n'"})
	assert.Equalf(t, CheckExitOK, res3.State, "state OK")
	assert.Contains(t, string(res3.BuildPluginOutput()), "OK - No matching lines found")

	res3 = snc.RunCheck("check_logfile", []string{"files=./t/test.log", "offset=0", "line-split='INFO'"})
	assert.Equalf(t, CheckExitOK, res3.State, "state OK")
	assert.Contains(t, string(res3.BuildPluginOutput()), "OK - 5 line(s) found")

	StopTestAgent(t, snc)
}

func TestCheckLogFileErrorOnEmptySearchPathResults(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)

	res := snc.RunCheck("check_logfile", []string{"file=./t/testfiledoesnotexist.log"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state should be UNKNOWN")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - no files found for search pattern: './t/testfiledoesnotexist.log'")

	res = snc.RunCheck("check_logfile", []string{"file=./t/patterndoesnotmatchanything*"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state should be UNKNOWN")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - no files found for search pattern: './t/patterndoesnotmatchanything*'")

	res = snc.RunCheck("check_logfile", []string{"files=./t/testfiledoesnotexist.log"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state should be UNKNOWN")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - no files found for search pattern: './t/testfiledoesnotexist.log'")

	res = snc.RunCheck("check_logfile", []string{"files=./t/test.log,./t/testfiledoesnotexist.log"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state should be UNKNOWN")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - no files found for search pattern: './t/testfiledoesnotexist.log'")

	res = snc.RunCheck("check_logfile", []string{"files=./t/test.log,./t/test2.log,./t/patterndoesnotmatchanything*"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state should be UNKNOWN")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - no files found for search pattern: './t/patterndoesnotmatchanything*'")

	StopTestAgent(t, snc)
}

func TestCheckLogFileErrorOnEmptySearchPathResults2(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)

	res := snc.RunCheck("check_logfile", []string{"file=./t/testfiledoesnotexist.log", "ignore-missing=1"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - No matching lines found")

	res = snc.RunCheck("check_logfile", []string{"file=./t/patterndoesnotmatchanything*", "ignore-missing=1"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - No matching lines found")

	res = snc.RunCheck("check_logfile", []string{"files=./t/testfiledoesnotexist.log", "ignore-missing=1"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - No matching lines found")

	res = snc.RunCheck("check_logfile", []string{"files=./t/test.log,./t/testfiledoesnotexist.log", "ignore-missing=1"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - 8 line(s) found")

	// need to reset offset due to previous calls
	res = snc.RunCheck("check_logfile", []string{"files=./t/test.log,./t/test2.log,./t/patterndoesnotmatchanything*", "ignore-missing=1", "offset=0"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - 16 line(s) found")

	StopTestAgent(t, snc)
}

func TestCheckLogFileAllowedPatternTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	require.NoError(t, os.MkdirAll(filepath.Join(logsDir, "sub"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "app.log"), []byte("allowed-line\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "secret.txt"), []byte("TOPSECRET-LEAK\n"), 0o600))

	snc := StartTestAgent(t, testLogfileAllowedPatternConfig(filepath.Join(logsDir, "**")))
	defer StopTestAgent(t, snc)

	// control: a file inside the allowed pattern is readable
	insideFile := filepath.Join(logsDir, "app.log")
	res := snc.RunCheck("check_logfile", []string{"files=" + insideFile, "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "inside file is allowed")
	assert.Contains(t, string(res.BuildPluginOutput()), "allowed-line")

	// control: a plain path outside the allowed pattern is rejected
	outsideFile := filepath.Join(tmpDir, "secret.txt")
	res = snc.RunCheck("check_logfile", []string{"files=" + outsideFile, "show-all"})
	assert.Equalf(t, CheckExitUnknown, res.State, "plain outside path is rejected")
	assert.Contains(t, string(res.BuildPluginOutput()), "does not match any allowed pattern")

	// a dot-dot path that stays inside the allowed pattern is still allowed
	insideTraversal := strings.Join([]string{logsDir, "sub", "..", "app.log"}, string(os.PathSeparator))
	res = snc.RunCheck("check_logfile", []string{"files=" + insideTraversal, "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "dot-dot inside the allowed pattern is allowed")
	assert.Contains(t, string(res.BuildPluginOutput()), "allowed-line")

	// a dot-dot path that resolves outside the allowed pattern must be rejected
	outsideTraversal := strings.Join([]string{logsDir, "..", "secret.txt"}, string(os.PathSeparator))
	res = snc.RunCheck("check_logfile", []string{"files=" + outsideTraversal, "show-all"})
	assert.Equalf(t, CheckExitUnknown, res.State, "dot-dot traversal is rejected")
	assert.Contains(t, string(res.BuildPluginOutput()), "does not match any allowed pattern")
	assert.NotContains(t, string(res.BuildPluginOutput()), "TOPSECRET-LEAK")
}

func TestCheckLogFileAllowedPatternSymlinkedDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test is unix specific")
	}

	tmpDir := t.TempDir()
	realLogsDir := filepath.Join(tmpDir, "real-logs")
	require.NoError(t, os.MkdirAll(realLogsDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(realLogsDir, "app.log"), []byte("allowed-line\n"), 0o600))

	linkedLogsDir := filepath.Join(tmpDir, "logs")
	require.NoError(t, os.Symlink(realLogsDir, linkedLogsDir))

	snc := StartTestAgent(t, testLogfileAllowedPatternConfig(filepath.Join(linkedLogsDir, "**")))
	defer StopTestAgent(t, snc)

	argumentFileName := filepath.Join(linkedLogsDir, "app.log")
	canonicalFileName, err := resolveCanonicalPath(argumentFileName)
	require.NoError(t, err)
	require.NotEqual(t, argumentFileName, canonicalFileName, "test requires a symlinked path")

	res := snc.RunCheck("check_logfile", []string{"files=" + argumentFileName})
	assert.Equalf(t, CheckExitOK, res.State, "file below the symlinked allowed pattern is allowed")

	// the filename attribute keeps the argument path, no canonical path is added
	require.NotNil(t, res.Raw)
	require.NotEmpty(t, res.Raw.listData)
	for _, entry := range res.Raw.listData {
		assert.Equal(t, argumentFileName, entry["filename"], "filename attribute keeps the argument path")
		assert.NotContains(t, entry, "canonical_filename", "no extra canonical attribute is added")
	}

	// the detail output shows the argument path, not the resolved path
	res = snc.RunCheck("check_logfile", []string{"files=" + argumentFileName, "offset=0", "show-all", "detail-syntax=$(filename)"})
	output := string(res.BuildPluginOutput())
	assert.Contains(t, output, argumentFileName)
	assert.NotContains(t, output, canonicalFileName)
}

func TestCheckLogFileAllowedPatternSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test is unix specific")
	}

	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "secret.txt"), []byte("TOPSECRET-LEAK\n"), 0o600))

	linkFileName := filepath.Join(logsDir, "leak.log")
	require.NoError(t, os.Symlink(filepath.Join(tmpDir, "secret.txt"), linkFileName))

	snc := StartTestAgent(t, testLogfileAllowedPatternConfig(filepath.Join(logsDir, "**")))
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_logfile", []string{"files=" + linkFileName, "show-all"})
	assert.Equalf(t, CheckExitUnknown, res.State, "symlink escaping the allowed pattern is rejected")
	assert.Contains(t, string(res.BuildPluginOutput()), "does not match any allowed pattern")
	assert.NotContains(t, string(res.BuildPluginOutput()), "TOPSECRET-LEAK")
}

func TestCheckLogFileResolveFailureIsRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test is unix specific")
	}

	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o750))

	brokenLink := filepath.Join(logsDir, "broken.log")
	require.NoError(t, os.Symlink(filepath.Join(tmpDir, "missing.log"), brokenLink))

	snc := StartTestAgent(t, testLogfileAllowedPatternConfig(filepath.Join(logsDir, "**")))
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_logfile", []string{"files=" + brokenLink, "show-all"})
	assert.Equalf(t, CheckExitUnknown, res.State, "unresolvable file fails closed")
	assert.Contains(t, string(res.BuildPluginOutput()), "failed to resolve file")
}

func TestCheckLogFileFilenameAttribute(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)
	defer StopTestAgent(t, snc)

	argumentFileName := "./t/test.log"
	res := snc.RunCheck("check_logfile", []string{"files=" + argumentFileName})
	require.NotNil(t, res.Raw)
	require.NotEmpty(t, res.Raw.listData)
	for _, entry := range res.Raw.listData {
		assert.Equal(t, argumentFileName, entry["filename"], "filename attribute keeps the argument path")
		assert.NotContains(t, entry, "canonical_filename", "no extra canonical attribute is added")
	}

	canonicalFileName, err := resolveCanonicalPath(argumentFileName)
	require.NoError(t, err)
	require.NotEqual(t, argumentFileName, canonicalFileName, "test requires a relative argument path")

	res = snc.RunCheck("check_logfile", []string{"files=" + argumentFileName, "offset=0", "show-all", "detail-syntax=$(filename)"})
	output := string(res.BuildPluginOutput())
	assert.Contains(t, output, argumentFileName)
	assert.NotContains(t, output, canonicalFileName)
}
