package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const testLogfileConfig = `
[/modules]
CheckLogFile = enabled

[/settings/check/logfile]
allowed pattern  = **
`

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
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - No files found to search lines in, search paths: './t/testfiledoesnotexist*'")

	StopTestAgent(t, snc)
}

func TestCheckLogFileFileDoesNotExist2(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)

	res := snc.RunCheck("check_logfile", []string{"file=./t/testfiledoesnotexist1", "files=./t/testfiledoesnotexist2,./t/testfiledoesnotexist3"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state should be UNKNOWN")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - No files found to search lines in, search paths: './t/testfiledoesnotexist1 , ./t/testfiledoesnotexist2 , ./t/testfiledoesnotexist3'")

	StopTestAgent(t, snc)
}

func TestCheckLogFileLineExistsOnTheFirstFile(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)

	res := snc.RunCheck("check_logfile", []string{"files=./t/test*", "filter='line LIKE testUser2'"})
	assert.Equalf(t, CheckExitOK, res.State, "state should be OK")
}

func TestCheckLogFileLineSplit(t *testing.T) {
	snc := StartTestAgent(t, testLogfileConfig)

	res := snc.RunCheck("check_logfile", []string{"files=./t/test.log", "offset=0"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - All 8 / 8 Lines OK")

	res2 := snc.RunCheck("check_logfile", []string{"files=./t/test.log", "offset=0", "line-split='\\n'"})
	assert.Equalf(t, CheckExitOK, res2.State, "state OK")
	assert.Contains(t, string(res2.BuildPluginOutput()), "OK - All 8 / 8 Lines OK")

	res3 := snc.RunCheck("check_logfile", []string{"files=./t/test.log", "offset=0", "line-split='\\r\\n'"})
	assert.Equalf(t, CheckExitOK, res3.State, "state OK")
	assert.Contains(t, string(res3.BuildPluginOutput()), "OK - All 8 / 8 Lines OK")

	res3 = snc.RunCheck("check_logfile", []string{"files=./t/test.log", "line-split='\\r\\n'"})
	assert.Equalf(t, CheckExitOK, res3.State, "state OK")
	assert.Contains(t, string(res3.BuildPluginOutput()), "OK - All 0 / 0 Lines OK")

	res3 = snc.RunCheck("check_logfile", []string{"files=./t/test.log", "offset=0", "line-split='INFO'"})
	assert.Equalf(t, CheckExitOK, res3.State, "state OK")
	assert.Contains(t, string(res3.BuildPluginOutput()), "OK - All 5 / 5 Lines OK")

	StopTestAgent(t, snc)
}
