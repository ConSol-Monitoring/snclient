//go:build !windows

package snclient

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckMailqPostfixJSON(t *testing.T) {
	snc := StartTestAgent(t, "")

	testModeFakeHasCapabilities = true
	defer func() { testModeFakeHasCapabilities = false }()

	tmpFolder := t.TempDir()
	MockSystemUtilities(t, map[string]string{"postconf": tmpFolder})

	err := os.Mkdir(filepath.Join(tmpFolder, "active"), 0o0700)
	require.NoError(t, err)
	err = os.Mkdir(filepath.Join(tmpFolder, "deferred"), 0o0700)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpFolder, "active", "testfile"), []byte("test"), 0o0600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpFolder, "deferred", "testfile"), []byte("test"), 0o0600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpFolder, "deferred", "testfile2"), []byte("test"), 0o0600)
	require.NoError(t, err)

	res := snc.RunCheck("check_mailq", []string{"mta=postfix"})
	assert.Equalf(t, CheckExitWarning, res.State, "state Warning")
	assert.Equalf(t,
		"WARNING - postfix: active 1 / deferred 2 |'active'=1;5;10;0 'active_size'=4B;10000000;20000000;0 'deferred'=2;0;10;0 'deferred_size'=8B;10000000;20000000;0",
		string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}
