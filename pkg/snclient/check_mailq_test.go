//go:build !windows

package snclient

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

//nolint:lll // preserve realistic postqueue JSON output with one queue entry per line
func TestCheckMailqPostfixJSON(t *testing.T) {
	snc := StartTestAgent(t, "")

	// replace absolute path for testing purpose
	testModeFakeHasCapabilities = true
	defer func() { testModeFakeHasCapabilities = false }()
	postQueuePath = "postqueue"

	// mock postqueue with anonymized output from a real postfix queue
	tmpPath := MockSystemUtilities(t, map[string]string{
		"postqueue": `
{"queue_name": "active", "queue_id": "AAAAAAAAAAA", "arrival_time": 1785318106, "message_size": 306, "forced_expire": false, "sender": "sender@example.invalid", "recipients": [{"orig_address": "", "address": "active@example.invalid"}]}
{"queue_name": "deferred", "queue_id": "BBBBBBBBBBB", "arrival_time": 1785318106, "message_size": 2552, "forced_expire": false, "sender": "MAILER-DAEMON", "recipients": [{"orig_address": "", "address": "deferred@example.invalid", "delay_reason": "alias database unavailable"}]}
{"queue_name": "hold", "queue_id": "CCCCCCCCCCC", "arrival_time": 1785258307, "message_size": 414, "forced_expire": false, "sender": "MAILER-DAEMON", "recipients": [{"orig_address": "", "address": "postqueue-test@example.invalid", "delay_reason": "Host or domain name not found"}]}`,
	})
	defer os.RemoveAll(tmpPath)

	res := snc.RunCheck("check_mailq", []string{"mta=postfix"})
	assert.Equalf(t, CheckExitWarning, res.State, "state Warning")
	assert.Equalf(t,
		"WARNING - postfix: active 1 / deferred 2 |'active'=1;5;10;0 'active_size'=306B;10000000;20000000;0 'deferred'=2;0;10;0 'deferred_size'=2966B;10000000;20000000;0",
		string(res.BuildPluginOutput()), "output matches")

	MockSystemUtilities(t, map[string]string{"postqueue": ""})
	res = snc.RunCheck("check_mailq", []string{"mta=postfix"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Equalf(t,
		"OK - postfix: active 0 / deferred 0 |'active'=0;5;10;0 'active_size'=0B;10000000;20000000;0 'deferred'=0;0;10;0 'deferred_size'=0B;10000000;20000000;0",
		string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}
