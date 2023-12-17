package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckAlias(t *testing.T) {
	config := `
[/modules]
CheckExternalScripts = enabled

[/settings/external scripts/alias]
alias_cpu = check_cpu warn=load=101 crit=load=102

[/settings/external scripts/alias/alias_cpu2]
command = check_cpu warn=load=101 crit=load=102
ignore perfdata = yes

[/settings/external scripts/alias/alias_cpu3]
command = check_cpu warn=load=101 crit=load=102
allow arguments = yes

[/settings/external scripts/alias/alias_cpu4]
command = check_cpu warn=load=101 crit=load=102
allow arguments = yes
nasty characters = []{}
`
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("alias_cpu", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		regexp.MustCompile(`^OK: CPU load is ok. \d+% on \d+ cores \|'total 5m'=\d+%;101;102 'total 1m'=\d+%;101;102 'total 5s'=\d+%;101;102$`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	// arguments not allowed
	res = snc.RunCheck("alias_cpu", []string{"argument"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state Unknown")
	assert.Equalf(t, "Exception processing request: Request contained arguments (check the allow arguments option).", res.Output, "plugin output")

	res = snc.RunCheck("alias_cpu2", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		regexp.MustCompile(`^OK: CPU load is ok. \d+% on \d+ cores \|'total 5m'=\d+%;101;102 'total 1m'=\d+%;101;102 'total 5s'=\d+%;101;102$`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	// arguments allowed
	res = snc.RunCheck("alias_cpu3", []string{"filter=none"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Containsf(t, res.Output, "OK: CPU load is ok.", "plugin output")

	// nasty char
	res = snc.RunCheck("alias_cpu3", []string{"filter=core!=$"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state Unknown")
	assert.Equalf(t, "Exception processing request: Request contained illegal characters (check the allow nasty characters option).", res.Output, "plugin output")

	// nasty char list changed
	res = snc.RunCheck("alias_cpu4", []string{"filter=core!=$"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")

	StopTestAgent(t, snc)
}
