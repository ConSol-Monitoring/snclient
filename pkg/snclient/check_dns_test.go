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

	t.Run("mx query", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "consol.de", "-q", "MX"})
		assert.Equalf(t, CheckExitOK, res.State, "state ok")
		assert.Regexpf(
			t,
			`^OK - consol\.de returns mail\.consol\.de\. \(MX\)`,
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

	t.Run("norec mode", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "--norec"})
		assert.Equalf(t, CheckExitOK, res.State, "state ok")
		assert.Regexpf(
			t,
			`^OK - labs\.consol\.de returns 94\.185\.89\.33`,
			string(res.BuildPluginOutput()),
			"output matches",
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

	StopTestAgent(t, snc)
}
