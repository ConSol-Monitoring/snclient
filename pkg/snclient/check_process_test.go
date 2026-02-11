//go:build !windows

package snclient

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckProcess(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_process", []string{})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `^OK - all \d+ processes are ok`, string(res.BuildPluginOutput()), "output matches")
	assert.Regexpf(t, `'count'=\d+;0;0;0$`, string(res.BuildPluginOutput()), "perfdata ok")

	res = snc.RunCheck("check_process", []string{"process=noneexisting.exe"})
	assert.Equalf(t, CheckExitCritical, res.State, "state critical")
	assert.Equalf(t, "CRITICAL - no processes found with this filter. |'count'=0;0;0;0 'rss'=0B;;;0 'virtual'=0B;;;0 'cpu'=0%;;;0",
		string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_process", []string{"process=noneexisting.exe", "ok=count=0"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Equalf(t, "OK - no processes found with this filter. |'count'=0;0;0;0 'rss'=0B;;;0 'virtual'=0B;;;0 'cpu'=0%;;;0",
		string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_process", []string{"process=noneexisting.exe", "empty-state=0"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Equalf(t, "OK - no processes found with this filter. |'count'=0;0;0;0 'rss'=0B;;;0 'virtual'=0B;;;0 'cpu'=0%;;;0",
		string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_process", []string{"process=noneexisting.exe", "crit=count>0", "warn=none"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `OK - no processes found with this filter.`, string(res.BuildPluginOutput()), "output ok")

	// get name of current process
	pid, err := convert.Int32E(os.Getpid())
	require.NoErrorf(t, err, "got own pid")
	me, err := process.NewProcess(pid)
	require.NoErrorf(t, err, "got own process")
	myExe, err := me.Exe()
	require.NoErrorf(t, err, "got own exe")
	res = snc.RunCheck("check_process", []string{"process=" + filepath.Base(myExe)})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `OK - all \d+ processes are ok.`, string(res.BuildPluginOutput()), "output ok")
	assert.Regexpf(t, `rss'=\d{1,15}B;;;0`, string(res.BuildPluginOutput()), "rss ok")

	// should work case insensitive
	res = snc.RunCheck("check_process", []string{"process=" + strings.ToUpper(filepath.Base(myExe))})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `OK - all \d+ processes are ok.`, string(res.BuildPluginOutput()), "output ok")
	assert.Regexpf(t, `rss'=\d{1,15}B;;;0`, string(res.BuildPluginOutput()), "rss ok")

	// check process it not running
	res = snc.RunCheck("check_process", []string{"process=nonexisting.exe", "crit=state='started'", "warn=none", "show-all", "empty-state=0"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `OK - no processes found with this filter`, string(res.BuildPluginOutput()), "output ok")
	assert.Regexpf(t, `'count'=0;`, string(res.BuildPluginOutput()), "count ok")

	// appending filter to previously set filter
	res = snc.RunCheck("check_process", []string{"filter='pid > 0'", "filter+='pid < 10000'"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `OK - all \d+ processes are ok`, string(res.BuildPluginOutput()), "output ok")

	StopTestAgent(t, snc)
}
