//go:build !windows

package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckDNS(t *testing.T) {
	config := `
[/modules]
CheckBuiltinPlugins = enabled
	`
	snc := StartTestAgent(t, config)

	t.Run("basic a lookup", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de"})
		assert.Equalf(t, CheckExitOK, res.State, "state ok")
		assert.Regexpf(
			t,
			`^OK - labs\.consol\.de returns 94\.185\.89\.33`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("expected string all match", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-e", "94.185.89.33"})
		assert.Equalf(t, CheckExitOK, res.State, "state ok")
		assert.Regexpf(
			t,
			`^OK - labs\.consol\.de returns 94\.185\.89\.33`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("expected string extra expected", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-e", "94.185.89.33", "-e", "1.2.3.4"})
		assert.Equalf(t, CheckExitWarning, res.State, "state warning")
		assert.Regexpf(
			t,
			`^WARNING - labs\.consol\.de returns 94\.185\.89\.33`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("expected string missing expected", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-e", "94.185.89.33", "-e", "1.2.3.4", "-e", "5.6.7.8"})
		assert.Equalf(t, CheckExitWarning, res.State, "state warning")
		assert.Regexpf(
			t,
			`^WARNING - labs\.consol\.de returns 94\.185\.89\.33`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("expected string none match", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-e", "1.2.3.4"})
		assert.Equalf(t, CheckExitCritical, res.State, "state critical")
		assert.Regexpf(
			t,
			`^CRITICAL - labs\.consol\.de returns 94\.185\.89\.33`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("warning threshold not triggered", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-w", "999"})
		assert.Equalf(t, CheckExitOK, res.State, "state ok")
		assert.Regexpf(
			t,
			`^OK`,
			string(res.BuildPluginOutput()),
			"not warned",
		)
	})

	t.Run("critical threshold triggered", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-c", "0"})
		assert.Equalf(t, CheckExitCritical, res.State, "state critical")
		assert.Regexpf(
			t,
			`^CRITICAL`,
			string(res.BuildPluginOutput()),
			"critical threshold triggered",
		)
	})

	t.Run("aaaa lookup", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-q", "AAAA"})
		assert.Equalf(t, CheckExitOK, res.State, "state ok")
		assert.Regexpf(
			t,
			`^OK - labs\.consol\.de returns 2a03:3680:0:2::21 \(AAAA\)`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("aaaa expected string match", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-q", "AAAA", "-e", "2a03:3680:0:2::21"})
		assert.Equalf(t, CheckExitOK, res.State, "state ok")
		assert.Regexpf(
			t,
			`^OK - labs\.consol\.de returns 2a03:3680:0:2::21`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("multiple answers all match", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "one.one.one.one", "-e", "1.1.1.1", "-e", "1.0.0.1"})
		assert.Equalf(t, CheckExitOK, res.State, "state ok")
		assert.Regexpf(
			t,
			`^OK - one\.one\.one\.one returns (1\.1\.1\.1|1\.0\.0\.1)`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	// Resolves cloudflares one.one.one.one using whatever nameserver is configured, not the 1.1.1.1 DNS namesever

	t.Run("multiple answers partial match", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "one.one.one.one", "-e", "1.1.1.1"})
		assert.Equalf(t, CheckExitWarning, res.State, "state warning")
		assert.Regexpf(
			t,
			`^WARNING - one\.one\.one\.one returns (1\.1\.1\.1|1\.0\.0\.1)`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	// Resolves cloudflares one.one.one.one using whatever nameserver is configured, not the 1.1.1.1 DNS namesever

	t.Run("multiple answers none match", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "one.one.one.one", "-e", "8.8.8.8"})
		assert.Equalf(t, CheckExitCritical, res.State, "state critical")
		assert.Regexpf(
			t,
			`^CRITICAL - one\.one\.one\.one returns (1\.1\.1\.1|1\.0\.0\.1)`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("missing host argument", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - .*host.*`,
			string(res.BuildPluginOutput()),
			"missing host argument",
		)
	})

	t.Run("empty host argument", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", ""})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - host must not be empty`,
			string(res.BuildPluginOutput()),
			"empty host argument",
		)
	})

	t.Run("empty query type", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-q", " "})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - query type must not be empty`,
			string(res.BuildPluginOutput()),
			"empty query type",
		)
	})

	t.Run("invalid port", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-p", "70000"})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - port must be between 1 and 65535, got: 70000`,
			string(res.BuildPluginOutput()),
			"invalid port",
		)
	})

	t.Run("zero timeout", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-t", "0"})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - timeout must be a positive number of seconds, got: 0`,
			string(res.BuildPluginOutput()),
			"zero timeout",
		)
	})

	t.Run("negative timeout", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-t", "-5"})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - timeout must be a positive number of seconds, got: -5`,
			string(res.BuildPluginOutput()),
			"negative timeout",
		)
	})

	t.Run("negative warning threshold", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-w", "-1"})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - warning threshold must not be negative, got: -1`,
			string(res.BuildPluginOutput()),
			"negative warning threshold",
		)
	})

	t.Run("negative critical threshold", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-c", "-1"})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - critical threshold must not be negative, got: -1`,
			string(res.BuildPluginOutput()),
			"negative critical threshold",
		)
	})

	t.Run("warning threshold higher than critical", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-w", "10", "-c", "5"})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - warning threshold \(10\) must not be higher than the critical threshold \(5\)`,
			string(res.BuildPluginOutput()),
			"warning threshold higher than critical",
		)
	})

	t.Run("empty expected string", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-e", ""})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - expected string must not be empty`,
			string(res.BuildPluginOutput()),
			"empty expected string",
		)
	})

	StopTestAgent(t, snc)
}
