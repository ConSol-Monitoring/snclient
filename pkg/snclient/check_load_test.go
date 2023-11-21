package snclient

import (
	"crypto/rand"
	"regexp"
	"testing"
	"time"

	"pkg/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckLoad(t *testing.T) {
	snc := StartTestAgent(t, "")

	// wait a couple of seconds if load is really zero
	// might happen on windows because load is meassured once the agent starts
	res := snc.RunCheck("check_load", []string{"warn=load > 0", "crit=load > 0"})
	if res.State == CheckExitOK {
		waitUntil := time.Now().Add(61 * time.Second)
		t.Logf("current load is zero, trying to utilize cpu a bit: %s", res.BuildPluginOutput())
		lastLogged := time.Now()
		for time.Now().Before(waitUntil) {
			for i := 1; i <= 10; i++ {
				b := make([]byte, 2000)
				_, err := rand.Read(b)
				require.NoErrorf(t, err, "rand.Read ok")
				_, err = utils.Sha256Sum(string(b))
				require.NoErrorf(t, err, "utils.Sha256Sum ok")
			}
			res := snc.RunCheck("check_load", []string{"warn=load > 0", "crit=load > 0"})
			if res.State != CheckExitOK {
				break
			}
			if lastLogged.Before(time.Now().Add(-3 * time.Second)) {
				t.Logf("current load still zero: %s", res.BuildPluginOutput())
				lastLogged = time.Now()
			}
		}
	}

	res = snc.RunCheck("check_load", []string{"warn=load > 0", "crit=load > 0"})
	assert.Equalf(t, CheckExitCritical, res.State, "state Critical")
	assert.Regexpf(t,
		regexp.MustCompile(`^CRITICAL: total load average: [\d\.]+, [\d\.]+, [\d\.]+ \|'load1'=[\d\.]+;0;0;0 'load5'=[\d\.]+;0;0;0 'load15'=[\d\.]+;0;0;0$`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_load", []string{"-w", "0,0,0", "-c", "999,998,997"})
	assert.Equalf(t, CheckExitWarning, res.State, "state Warning")
	assert.Regexpf(t,
		regexp.MustCompile(`^WARNING: total load average: [\d\.]+, [\d\.]+, [\d\.]+ \|'load1'=[\d\.]+;0;999;0 'load5'=[\d\.]+;0;998;0 'load15'=[\d\.]+;0;997;0$`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	StopTestAgent(t, snc)
}
