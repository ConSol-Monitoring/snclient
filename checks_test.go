package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckSNClientVersion(t *testing.T) {
	snc := Agent{}
	res := snc.RunCheck("check_snclient_version", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		regexp.MustCompile(`^SNClient\+ v\d+`),
		string(res.BuildPluginOutput()),
		"output matches",
	)
}

func TestCheckCPU(t *testing.T) {
	snc := StartTestAgent(t, "", []string{})

	res := snc.RunCheck("check_cpu", []string{"warn=load = 99", "crit=load = 100"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		regexp.MustCompile(`^OK: CPU load is ok.\|'total 5m'=\d+%;99;100 'total 1m'=\d+%;99;100 'total 5s'=\d+%;99;100$`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	StopTestAgent(t, snc)
}
