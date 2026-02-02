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
allow arguments = true

[/settings/external scripts/scripts]
check_files_recursive_generate_files = ./check_files_recursive_generate_files.sh "$ARG1$"
check_files_perfdata_generate_files = ./check_files_perfdata_generate_files.sh "$ARG1$"

[/settings/external scripts/scripts/check_files_perfdata_generate_files]
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

	res := snc.RunCheck("check_files_recursive_generate_files", []string{geneartionDirectory})
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

func TestCheckFilesSizePerfdata(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := checkFilesTestConfig(t, scriptsDir)
	snc := StartTestAgent(t, config)

	// capture this since t.TempDir() is dyanic
	geneartionDirectory := t.TempDir()

	res := snc.RunCheck("check_files_perfdata_generate_files", []string{geneartionDirectory})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	outputString := string(res.BuildPluginOutput())
	assert.Containsf(t, outputString, "ok - Generated 16 files for testing", "output matches")
	assert.Containsf(t, outputString, "ok - Generated 2 directories for testing", "output matches")

	// Total size should be exactly 14 mb
	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "crit='total_size > 100Mb'"})
	outputString = string(res.BuildPluginOutput())
	// This check includes the directories as well
	assert.Containsf(t, outputString, "OK - All 18 files are ok", "output matches")

	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "crit='total_size > 100Mb'", "filter='type == file'"})
	outputString = string(res.BuildPluginOutput())
	// This filters out the two directories
	assert.Containsf(t, outputString, "OK - All 16 files are ok", "output matches")
	assert.Contains(t, outputString, "'total_size'=14680064B")

	// only match the root files and check their total size
	// file_1024kb_1.root
	// file_1024kb_2.root
	// file_1024kb_3.root
	// file_512kb_1.root
	// file_512kb_2.root
	// file_512kb_3.root
	// file_512kb_4.root
	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "max-depth=0", "crit='total_size > 100Mb'", "filter='type == file'"})
	outputString = string(res.BuildPluginOutput())
	assert.Containsf(t, outputString, "OK - All 7 files are ok", "output matches")
	assert.Contains(t, outputString, "'total_size'=5242880B")

	// only match the root files, but return critical if size is above 512kb
	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "max-depth=0", "crit='size > 512Kb'", "filter='type == file'"})
	outputString = string(res.BuildPluginOutput())
	assert.Containsf(t, outputString, "CRITICAL - 3/7 files (5.00 MiB)", "output matches")
	assert.Containsf(t, outputString, "file_1024kb_1.root size'=1048576B", "Should include metrics named '<filename> size' for all files")

	// "file_512kb_1.root",
	// "file_1024kb_1.root",
	// "a/file_1024kb_1.a",
	// "b/file_1024kb_1.b",
	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "pattern=file_*_1.*", "crit='total_size != 3670016'", "filter='type == file'"})
	outputString = string(res.BuildPluginOutput())
	// the metric check for 'total_size' sets the status to critical
	// files do not have a 'total_size' attribute, so they do not result towards setting the status
	assert.Containsf(t, outputString, "CRITICAL - 0/4 files (3.50 MiB)", "output matches")
	assert.Contains(t, outputString, "total_size'=3670016")

	// Search on the root, but use pattern that only matches files with "b" extension
	// The pattern matching should remove the files with "root" and "a" extensions
	// The second pass should remove the "a" folder where files with "a" extension is found
	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "pattern=*.b", "crit='size == 0B'", "filter='type == file'"})
	outputString = string(res.BuildPluginOutput())
	assert.Containsf(t, outputString, "OK - All 5 files are ok", "output matches")
	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "pattern=*.b", "crit='size == 0B'", "filter=' type == file'"})
	outputString = string(res.BuildPluginOutput())
	assert.Containsf(t, outputString, "OK - All 5 files are ok", "output matches")

	// check if the search_path attribute is being populated
	aDirectory := filepath.Join(geneartionDirectory, "a")
	bDirectory := filepath.Join(geneartionDirectory, "b")
	res = snc.RunCheck("check_files", []string{"paths=" + aDirectory + "," + bDirectory, "critical='check_path != " + aDirectory + " '"})
	outputString = string(res.BuildPluginOutput())
	assert.Containsf(t, outputString, "CRITICAL - 5/9 files", "output matches")
	assert.NotContainsf(t, outputString, "file_1024kb_1.a", "output matches")

	// check if calculate-subdirectory-sizes works
	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "crit='size > 4Mib'", "calculate-subdirectory-sizes=true"})
	outputString = string(res.BuildPluginOutput())
	// files themselves are all 512 Kib or 1 Mib
	// directory 'a' contains four 1 MiB files,
	// the directory 'b' contains five 1MiB files, it is the only candidate to go over 4MiB
	assert.Containsf(t, outputString, "CRITICAL - 1/18 files (14.00 MiB) critical(b)", "output matches")

	// When using a pattern and calculate subdirectory sizes is enabled, it will add the subdirectory sizes as metrics
	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "pattern=*_3.*", "crit='size > 0Mib'", "calculate-subdirectory-sizes=true"})
	outputString = string(res.BuildPluginOutput())
	assert.Containsf(t, outputString, "CRITICAL - 6/6 files (3.50 MiB) critical", "output matches")
	assert.Containsf(t, outputString, "'a size'=1048576B", "should calculate the size of the subfolder a")
	assert.Containsf(t, outputString, "'b size'=1048576B", "should calculate the size of the subfolder b")

	StopTestAgent(t, snc)
}
