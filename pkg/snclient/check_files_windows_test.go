package snclient

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func checkFilesTestConfig(t *testing.T, scriptsDir string) string {
	t.Helper()

	config := fmt.Sprintf(`
[/modules]
CheckExternalScripts = enabled

[/paths]
scripts = %s
shared-path = %%(scripts)

[/settings/external scripts]
timeout = 1111111

[/settings/external scripts/scripts]
check_files_recurisve_generate_files = ./check_files_recursive_generate_files.ps1 "$ARG1$"

[/settings/external scripts/scripts/check_files_recurisve_generate_files]
allow arguments = true
allow nasty characters = true
`, scriptsDir)

	return config
}

func TestCheckFilesRecursiveArguments(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := checkFilesTestConfig(t, scriptsDir)
	snc := StartTestAgent(t, config)

	// capture this since t.TempDir() is dyanic
	geneartionDirectory := t.TempDir()

	res := snc.RunCheck("check_files_recurisve_generate_files", []string{geneartionDirectory})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")

	assert.Containsf(t, string(res.BuildPluginOutput()), "ok - Generated 11 files for testing", "output matches")
	assert.Containsf(t, string(res.BuildPluginOutput()), "ok - Generated 6 directories for testing", "output matches")

	// The script should build a file three that looks like this
	// /tmp/TestCheckFilesRecursiveArguments86871339/003
	// ├── directory1
	// │   ├── directory1-directory2
	// │   │   ├── directory1-directory2-directory3
	// │   │   │   └── directory1-directory2-directory3-file8
	// │   │   ├── directory1-directory2-file5
	// │   │   ├── directory1-directory2-file6
	// │   │   └── directory1-directory2-file7
	// │   ├── directory1-file3.txt
	// │   └── directory1-file4
	// ├── directory4
	// │   ├── directory4-directory5
	// │   │   └── directory4-directory5-file11
	// │   ├── directory4-directory6
	// │   ├── directory4-file10.html
	// │   └── directory4-file9.exe
	// ├── file1.txt
	// └── file2

	// No arguments: Recursion enabled. Should find everything under the folder.
	// 11 files and 6 folders => Reports 17 files
	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 17 files are ok", "output matches")

	// Max-depth=0
	// Depth is calculated by the amount of seperators from the base path
	// So only the path itself is going to have depth 0. Anything under it requires an appended /
	// But the check will always include files directly under it
	// file1.txt , file2 , directory1 , directory2
	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "max-depth=0"})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 4 files are ok", "output matches")

	// Max-depth=1
	// These items fit the depth condition, as they require one more separator to type after the base path
	// file1.txt , file2 , directory1 , directory2
	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "max-depth=1"})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 4 files are ok", "output matches")

	// Max-depth=2
	// file1.txt , file2 , directory1 , directory1-file3.txt, directory1-file4 , directory1-directory2, directory4,
	// directory4-file9.exe , directory4-file10.html , directory4-directory5 , directory4-directory6
	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "max-depth=2"})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 11 files are ok", "output matches")

	// Max-depth=3
	// Adds 5 more: directory1-directory2-directory3, directory1-directory2-file5 , directory1-directory2-file6, directory1-directory2-file7, directory4-directory5-file11
	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "max-depth=3"})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 16 files are ok", "output matches")

	// Max-depth=4
	// Adds the last file: directory1-directory2-directory3-file8
	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "max-depth=4"})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 17 files are ok", "output matches")

	// File and directory type checks
	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "filter='type=file'"})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 11 files are ok", "output matches")

	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "filter='type=dir'"})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 6 files are ok", "output matches")

	StopTestAgent(t, snc)
}

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
