package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckLogFile(t *testing.T) {
	snc := StartTestAgent(t, "")
	res := snc.RunCheck("check_logfile", []string{"file=./t/test.log"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK")

	res = snc.RunCheck("check_logfile", []string{"files=./t/test*"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "All 16")

	StopTestAgent(t, snc)
}

func TestCheckLogFilePathWildCards(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_logfile", []string{"files=./t/test*", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK")

	snc.alreadyParsedLogfiles.Clear()

	res = snc.RunCheck("check_logfile", []string{"files=./t/test*", "warn=line LIKE WARNING"})
	assert.Equalf(t, CheckExitWarning, res.State, "state WARNING")
	assert.Contains(t, string(res.BuildPluginOutput()), "WARNING - 2/16")

	res = snc.RunCheck("check_logfile", []string{"files=./t/test*", "warn=line LIKE WARNING"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - All 0 / 0")

	StopTestAgent(t, snc)
}

func TestCheckLogFileOKPatternResetsErrors(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_logfile", []string{"files=./t/test*", "warn=line LIKE ERROR", "ok=line LIKE 'System check completed successfully'"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "All 16")

	StopTestAgent(t, snc)
}

func TestCheckLogFileFilter(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_logfile", []string{"files=./t/test*", "filter=line LIKE 'WARNING'", "warn=count>1"})
	assert.Equalf(t, CheckExitWarning, res.State, "state WARNING")
	assert.Contains(t, string(res.BuildPluginOutput()), "WARNING - 0/2 ")

	StopTestAgent(t, snc)
}

func TestCheckLogfileLabel(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_logfile", []string{"files=./t/test*", "label='YEAR:^\\d{4}'", "label='ERWAR:(ERROR|WARN)'", "show-all", "detail-syntax=$(ERWAR)$(YEAR)- $(line:cut=50)"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - 2023")

	StopTestAgent(t, snc)
}

func TestCheckLogFileColumnN(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_logfile", []string{"files=./t/test*", "crit=column1 LIKE DEBUG", "column-split=;", "show-all"})
	assert.Equalf(t, CheckExitCritical, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "CRITICAL - ")

	StopTestAgent(t, snc)
}
