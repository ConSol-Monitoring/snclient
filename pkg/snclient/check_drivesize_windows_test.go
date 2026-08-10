//go:build windows

package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckDrivesize(t *testing.T) {
	snc := StartTestAgent(t, "")

	// This is a untypical behavior. if a drive= or folder= argument is given, it should be visible again in the output.
	// This means the default detail syntax that uses "drive" and "drive_or_name" attribute should be set to the argument.
	// So the lowercase print of the drive letters is not done in default cases like the ones bellow.
	res := snc.RunCheck("check_drivesize", []string{"warn=free > 0", "crit=free > 0", "drive=c"})
	assert.Equalf(t, CheckExitCritical, res.State, "state critical")
	assert.Regexpf(
		t,
		`^CRITICAL - C:\\ .*?\/.*? \(\d+\.\d+%\) \|'c: free'=.*?B;0;0;0;.*? 'c: free %'=.*?%;0;0;0;100`,
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_drivesize", []string{"filter=free<0", "empty-state=0"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - No drives found", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=free>0", "total"})
	assert.Contains(t, string(res.BuildPluginOutput()), "C:\\ used %", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "total free", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), "C:\\ free", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), ";0;;0;100", "output matches")

	res = snc.RunCheck("check_drivesize", []string{
		"warning=used > 99",
		"crit=used > 99.5",
		"empty-state=unknown",
		`filter=type in ('fixed') AND mounted=1 AND name not like '\?\'`,
		"show-all",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - C:\\ ", "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), ";99;99.5;0;100", "output matches")

	// test all variants of drive names

	// rules for status line and perfdata label:
	// use the string user gave as drive, do not change uppercase and lowercase,
	// add a colon if its missing
	// flip the slash to be a backwards slash

	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "drive=c", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `^OK - C:\\ .*?\/.*? \(\d+\.\d+%\) \|'c: used'=.*?B;(\d+);(\d+);0;(\d+) 'c: used %'=.*?%;100;100;0;100`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "drive=c:", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `^OK - C:\\ .*?\/.*? \(\d+\.\d+%\) \|'c: used'=.*?B;(\d+);(\d+);0;(\d+) 'c: used %'=.*?%;100;100;0;100`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "drive=c:\\", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `^OK - C:\\ .*?\/.*? \(\d+\.\d+%\) \|'c:\\ used'=.*?B;(\d+);(\d+);0;(\d+) 'c:\\ used %'=.*?%;100;100;0;100`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "drive=c:\\\\\\", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `^OK - C:\\ .*?\/.*? \(\d+\.\d+%\) \|'c:\\ used'=.*?B;(\d+);(\d+);0;(\d+) 'c:\\ used %'=.*?%;100;100;0;100`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "drive=c:/", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `^OK - C:\\ .*?\/.*? \(\d+\.\d+%\) \|'c:\\ used'=.*?B;(\d+);(\d+);0;(\d+) 'c:\\ used %'=.*?%;100;100;0;100`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "drive=c:///////", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `^OK - C:\\ .*?\/.*? \(\d+\.\d+%\) \|'c:\\ used'=.*?B;(\d+);(\d+);0;(\d+) 'c:\\ used %'=.*?%;100;100;0;100`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "drive=C", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `^OK - C:\\ .*?\/.*? \(\d+\.\d+%\) \|'C: used'=.*?B;(\d+);(\d+);0;(\d+) 'C: used %'=.*?%;100;100;0;100`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "drive=C:", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `^OK - C:\\ .*?\/.*? \(\d+\.\d+%\) \|'C: used'=.*?B;(\d+);(\d+);0;(\d+) 'C: used %'=.*?%;100;100;0;100`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "drive=C:\\", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `^OK - C:\\ .*?\/.*? \(\d+\.\d+%\) \|'C:\\ used'=.*?B;(\d+);(\d+);0;(\d+) 'C:\\ used %'=.*?%;100;100;0;100`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "drive=C:\\\\\\", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `^OK - C:\\ .*?\/.*? \(\d+\.\d+%\) \|'C:\\ used'=.*?B;(\d+);(\d+);0;(\d+) 'C:\\ used %'=.*?%;100;100;0;100`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "drive=C:/", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `^OK - C:\\ .*?\/.*? \(\d+\.\d+%\) \|'C:\\ used'=.*?B;(\d+);(\d+);0;(\d+) 'C:\\ used %'=.*?%;100;100;0;100`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "drive=C:///", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `^OK - C:\\ .*?\/.*? \(\d+\.\d+%\) \|'C:\\ used'=.*?B;(\d+);(\d+);0;(\d+) 'C:\\ used %'=.*?%;100;100;0;100`, string(res.BuildPluginOutput()), "output matches")

	// must not match
	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "drive=c:\\Windows"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state UNKNOWN")
	assert.Contains(t, string(res.BuildPluginOutput()), `not mounted`, "output matches")

	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "folder=c:\\Windows"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), `OK - All 1 drive`, "output matches")
	assert.Contains(t, string(res.BuildPluginOutput()), `c:\Windows used %`, "output matches")

	// check with forward slash
	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "folder=c:/Windows"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "drive=c:/"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")

	StopTestAgent(t, snc)
}

func TestNonexistingDrive(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_drivesize", []string{"drive=X"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - No drives found", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"drive=X:", "empty-state=warn"})
	assert.Equalf(t, CheckExitWarning, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "WARNING - No drives found", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"drive=X:\\", "empty-state=warn"})
	assert.Equalf(t, CheckExitWarning, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "WARNING - No drives found", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"drive=X", "empty-state=warn"})
	assert.Equalf(t, CheckExitWarning, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "WARNING - No drives found", "output matches")

	res = snc.RunCheck("check_drivesize", []string{"drive=X", "empty-state=crit"})
	assert.Equalf(t, CheckExitCritical, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "CRITICAL - No drives found", "output matches")

	StopTestAgent(t, snc)
}

func TestIsNetworkSharePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{`\\server\share`, true},
		{`//server/share`, true},
		{`\\server`, true},
		{`C:\folder`, false},
		{`C:`, false},
		{`/`, false},
		{``, false},
	}
	for _, test := range tests {
		assert.Equalf(t, test.want, isNetworkSharePath(test.path), "isNetworkSharePath(%q)", test.path)
	}
}

func TestIsHiddenSharePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{`\\server\C$`, true},
		{`\\server\ADMIN$`, true},
		{`\\server\share$`, true},
		{`\\server\share$\folder`, true},
		{`\\server\share`, false},
		{`\\server\share\folder`, false},
		{`\\server`, false},
		{`C:\folder`, false},
		{``, false},
	}
	cd := CheckDrivesize{}
	for _, test := range tests {
		assert.Equalf(t, test.want, cd.isHiddenSharePath(test.path), "isHiddenSharePath(%q)", test.path)
	}
}

func TestShareRoot(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{`\\server\share`, `\\server\share`},
		{`\\server\share\folder`, `\\server\share`},
		{`\\server\share\folder\file.txt`, `\\server\share`},
		{`\\server`, `\\server`},
		{`C:\folder`, `C:\folder`},
	}
	cd := CheckDrivesize{}
	for _, test := range tests {
		assert.Equalf(t, test.want, cd.shareRoot(test.path), "shareRoot(%q)", test.path)
	}
}

func TestMatchNetworkShare(t *testing.T) {
	shares := map[string]map[string]string{
		`Z:\`: {
			"remote_name": `\\server\share`,
			"drive":       `Z:\`,
			"connected":   "1",
		},
		`Y:\`: {
			"remote_name": `\\server\share2`,
			"drive":       `Y:\`,
			"connected":   "1",
		},
		`X:\`: {
			"remote_name": `\\offline\share`,
			"drive":       `X:\`,
			"connected":   "0",
		},
	}
	checkDrivesize := CheckDrivesize{}

	// exact match
	key, entry, matched := checkDrivesize.matchNetworkShare(`\\server\share`, shares)
	assert.Truef(t, matched, "exact match")
	assert.Equalf(t, `Z:\`, key, "exact match key")
	assert.Equalf(t, `Z:\`, entry["drive"], "exact match entry")

	// subfolder match
	key, _, matched = checkDrivesize.matchNetworkShare(`\\server\share\folder`, shares)
	assert.Truef(t, matched, "subfolder match")
	assert.Equalf(t, `Z:\`, key, "subfolder match key")

	// trailing backslash
	key, _, matched = checkDrivesize.matchNetworkShare(`\\server\share\`, shares)
	assert.Truef(t, matched, "trailing backslash match")
	assert.Equalf(t, `Z:\`, key, "trailing backslash match key")

	key, _, matched = checkDrivesize.matchNetworkShare(`\\server\share\\\\`, shares)
	assert.Truef(t, matched, "trailing backslash match")
	assert.Equalf(t, `Z:\`, key, "multiple trailing backslash match key")

	// case-insensitive
	key, _, matched = checkDrivesize.matchNetworkShare(`\\SERVER\SHARE\Folder`, shares)
	assert.Truef(t, matched, "case-insensitive match")
	assert.Equalf(t, `Z:\`, key, "case-insensitive match key")

	// prefix without share name boundary must not match
	_, _, matched = checkDrivesize.matchNetworkShare(`\\server\shareX`, shares)
	assert.Falsef(t, matched, "no match for share without boundary")

	key, _, matched = checkDrivesize.matchNetworkShare(`\\server\share2`, shares)
	assert.Truef(t, matched, "share2 matches its own remote name")
	assert.Equalf(t, `Y:\`, key, "share2 key")

	key, _, matched = checkDrivesize.matchNetworkShare(`\\server\share2\folder`, shares)
	assert.Truef(t, matched, "share2 subfolder matches")
	assert.Equalf(t, `Y:\`, key, "share2 subfolder key")

	// disconnected persistent drives are skipped
	_, _, matched = checkDrivesize.matchNetworkShare(`\\offline\share`, shares)
	assert.Falsef(t, matched, "disconnected drive skipped")

	// no match at all
	_, _, matched = checkDrivesize.matchNetworkShare(`\\other\share`, shares)
	assert.Falsef(t, matched, "no match")
}

func TestCleanupPathString(t *testing.T) {
	tests := []struct {
		path    string
		cleaned string
		isDrive bool
	}{
		{`c`, `C:`, true},
		{`c:`, `C:`, true},
		{`c:\`, `C:\`, true},
		{`C:\`, `C:\`, true},
		{`c:/`, `C:\`, true},
		{`c:\\`, `C:\`, true},
		{`C://///`, `C:\`, true},
		{`\\server\share`, `\server\share`, false},
	}
	cd := CheckDrivesize{}
	for _, test := range tests {
		cleaned, isDrive, err := cd.cleanupPathString(test.path)
		assert.NoErrorf(t, err, "cleanupPathString(%q)", test.path)
		assert.Equalf(t, test.cleaned, cleaned, "cleanupPathString(%q)", test.path)
		assert.Equalf(t, test.isDrive, isDrive, "cleanupPathString(%q) isDrive", test.path)
	}
}

func TestEnsureTrailingBackslash(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{`\\server\share`, `\\server\share\`},
		{`\\server\share\`, `\\server\share\`},
		{`\\server\share\\`, `\\server\share\`},
		{`\\server\share\\\\\\\\`, `\\server\share\`},
		{`\\server\share$\folder`, `\\server\share$\folder\`},
		{`C:`, `C:\`},
		{`C:\`, `C:\`},
		{`Z:\`, `Z:\`},
	}
	cd := CheckDrivesize{}
	for _, test := range tests {
		assert.Equalf(t, test.want, cd.ensureTrailingBackslash(test.path), "ensureTrailingBackslash(%q)", test.path)
	}
}
