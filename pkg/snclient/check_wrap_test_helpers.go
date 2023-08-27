package snclient

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setupTeardown(_ *testing.T, arg string) func() {
	// teardown function
	return func() {
		err := os.RemoveAll(arg)
		if err != nil {
			return
		}
	}
}

func runTestCheckExternalDefault(t *testing.T, snc *Agent) {
	t.Helper()

	res := snc.RunCheck("check_dummy", []string{})
	assert.Equalf(t, CheckExitUnknown, res.State, "state matches")
	assert.Equalf(t, "Invalid state argument. Please provide one of: 0, 1, 2, 3", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_ok", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "OK: i am ok", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_ok", []string{"0", "i am ok ignored"})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "OK: i am ok", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_critical", []string{})
	assert.Equalf(t, CheckExitCritical, res.State, "state matches")
	assert.Equalf(t, "CRITICAL: i am critical", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_critical", []string{"2", "i am critical ignored"})
	assert.Equalf(t, CheckExitCritical, res.State, "state matches")
	assert.Equalf(t, "CRITICAL: i am critical", string(res.BuildPluginOutput()), "output matches")

	// providing arguments here does not replace the arguments in the ini
	res = snc.RunCheck("check_dummy_unknown", []string{"3", "i am unknown ignored"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state matches")
	assert.Equalf(t, "UNKNOWN", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_arg", []string{"0", "arg ok"})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "OK: arg ok", string(res.BuildPluginOutput()), "output matches")
}

func runTestCheckExternalArgs(t *testing.T, snc *Agent) {
	t.Helper()

	res := snc.RunCheck("check_dummy_args", []string{"0", "arg ok"})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	// $ARGS$ makes "0", "arg ok" a flat list: 0 arg ok
	assert.Equalf(t, "OK: arg", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_args%", []string{"0", "arg ok"})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	// %ARGS% makes "0", "arg ok" a flat list: 0 arg ok
	assert.Equalf(t, "OK: arg", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_argsq", []string{"0", "arg ok"})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	// $ARG"S$ makes "0", "arg ok" a quoted list: "0" "arg ok"
	assert.Equalf(t, "OK: arg ok", string(res.BuildPluginOutput()), "output matches")
}

func runTestCheckExternalWrapped(t *testing.T, snc *Agent) {
	t.Helper()

	res := snc.RunCheck("check_dummy_wrapped_noparm", []string{"0", "i am wrapped"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state matches")
	assert.Equalf(t, "Invalid state argument. Please provide one of: 0, 1, 2, 3", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_wrapped", []string{"0", "i am wrapped"})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "OK: i am wrapped", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_wrapped_ok", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	assert.Equalf(t, "OK: i am ok wrapped", string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_dummy_wrapped_critical", []string{"0", "critical ignored"})
	assert.Equalf(t, CheckExitCritical, res.State, "state matches")
	assert.Equalf(t, "CRITICAL: i am critical wrapped", string(res.BuildPluginOutput()), "output matches")
}
