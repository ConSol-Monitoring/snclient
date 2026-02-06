package snclient

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckDriveLetterPaths(t *testing.T) {
	snc := StartTestAgent(t, "")
	var res *CheckResult

	// ===============  C:\pagefile.sys
	// Virtual memory is generally enabled, so this is safe to test locally.
	// But it might not exist on non-standard systems, like the Github Actions CI.
	_, pagefileCheckErr := os.Stat(`C:\pagefile.sys`)
	if pagefileCheckErr == nil {
		res = snc.RunCheck("check_files", []string{"path=C:", "max-depth=0", "filter= type == 'file' and name == 'pagefile.sys' "})
		assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 1 files are ok", "output matches")
		res = snc.RunCheck("check_files", []string{"path=C:", "max-depth=1", "filter= type == 'file' and name == 'pagefile.sys' "})
		assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 1 files are ok", "output matches")
	} else {
		// Skip pagefile tests if file doesn't exist
		t.Log("Skipping pagefile.sys tests - file not found")
	}

	// ===============  C:\Windows
	res = snc.RunCheck("check_files", []string{"path=C:", "max-depth=0", "filter= type == 'dir' and name == 'Windows' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 1 files are ok", "output matches")
	res = snc.RunCheck("check_files", []string{"path=C:", "max-depth=1", "filter= type == 'dir' and name == 'Windows' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 1 files are ok", "output matches")

	// ===============  C:\Windows\explorer.exe
	// max-depth=0 looks for items directly under the path, so it should not work
	res = snc.RunCheck("check_files", []string{"path=C:", "max-depth=0", "filter= type == 'file' and name == 'explorer.exe' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "No files found", "output matches")
	// max-depth=1 looks for items directly under the path, so it should not work
	res = snc.RunCheck("check_files", []string{"path=C:", "max-depth=1", "filter= type == 'file' and name == 'explorer.exe' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "No files found", "output matches")
	// There are two separators here, so the max-depth=2 should work
	res = snc.RunCheck("check_files", []string{"path=C:", "max-depth=2", "filter= type == 'file' and name == 'explorer.exe' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 1 files are ok", "output matches")

	// ===============  C:\Windows\notepad.exe
	// max-depth=0 looks for items directly under the path, so it should not work
	res = snc.RunCheck("check_files", []string{"path=C:", "max-depth=0", "filter= type == 'file' and name == 'notepad.exe' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "No files found", "output matches")
	// max-depth=1 looks for items directly under the path, so it should not work
	res = snc.RunCheck("check_files", []string{"path=C:", "max-depth=1", "filter= type == 'file' and name == 'notepad.exe' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "No files found", "output matches")
	// There are two separators here, so the max-depth=2 should work
	res = snc.RunCheck("check_files", []string{"path=C:", "max-depth=2", "filter= type == 'file' and name == 'notepad.exe' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 1 files are ok", "output matches")

	// ===============  C:\Windows\System32\cmd.exe
	// max-depth=0 looks for items directly under the path, so it should not work
	res = snc.RunCheck("check_files", []string{"path=C:", "max-depth=0", "filter= type == 'file' and name == 'cmd.exe' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "No files found", "output matches")
	// max-depth=1 looks for items directly under the path, so it should not work
	res = snc.RunCheck("check_files", []string{"path=C:", "max-depth=1", "filter= type == 'file' and name == 'cmd.exe' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "No files found", "output matches")
	// There are two seperators here, so the max-depth=2 should not work
	res = snc.RunCheck("check_files", []string{"path=C:", "max-depth=2", "filter= type == 'file' and name == 'cmd.exe' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "No files found", "output matches")
	// There are three separators here, so the maxx-depth=3 should work. But that catches the C:\Windows\SysWow64\cmd.exe as well
	res = snc.RunCheck("check_files", []string{"path=C:", "max-depth=3", "filter= type == 'file' and name == 'cmd.exe' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 2 files are ok", "output matches")

	// Path is given as C:\Windows. The file is not directly under there
	res = snc.RunCheck("check_files", []string{"path=C:\\Windows", "max-depth=0", "filter= type == 'file' and name == 'cmd.exe' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "No files found", "output matches")
	res = snc.RunCheck("check_files", []string{"path=C:\\Windows", "max-depth=1", "filter= type == 'file' and name == 'cmd.exe' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "No files found", "output matches")
	res = snc.RunCheck("check_files", []string{"path=C:\\Windows", "max-depth=2", "filter= type == 'file' and name == 'cmd.exe' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 2 files are ok", "output matches")

	// Path is given as C:\Windows\System32\
	res = snc.RunCheck("check_files", []string{"path=C:\\Windows", "max-depth=0", "filter= type == 'file' and name == 'cmd.exe' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "No files found", "output matches")

	// ===============  C:\Windows\Fonts\arial.ttf
	res = snc.RunCheck("check_files", []string{"path=C:", "max-depth=3", "filter= type == 'file' and name == 'arial.ttf' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 1 files are ok", "output matches")
	// Path can be given as C:\ as well
	res = snc.RunCheck("check_files", []string{"path=C:\\", "max-depth=3", "filter= type == 'file' and name == 'arial.ttf' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 1 files are ok", "output matches")

	StopTestAgent(t, snc)
}
