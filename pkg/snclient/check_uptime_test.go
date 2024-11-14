package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckUptime(t *testing.T) {
	snc := Agent{}
	res := snc.RunCheck("check_uptime", []string{})
	assert.Regexpf(t,
		`^\w+ - uptime:.*?(\d+w \d+d|\d+:\d+h|\d+m \d+s|\d+m|\d+s), boot: \d+\-\d+\-\d+ \d+:\d+:\d+ \(UTC\) \|'uptime'=\d+s;172800:;86400:`,
		string(res.BuildPluginOutput()),
		"output matches",
	)
}
