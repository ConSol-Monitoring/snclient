package snclient

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckFiles(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_files", []string{})
	assert.Equalf(t, CheckExitUnknown, res.State, "state Unknown")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - no path specified")

	res = snc.RunCheck("check_files", []string{"path=."})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "files are ok")

	res = snc.RunCheck("check_files", []string{"path=.", "crit=count>10000", "max-depth=1"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "'count'=")

	res = snc.RunCheck("check_files", []string{"path=.", "max-depth=0"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.NotContains(t, string(res.BuildPluginOutput()), "No files found")

	res = snc.RunCheck("check_files", []string{"paths= ., t", "crit=count>10000", "max-depth=1"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "'count'=")

	res = snc.RunCheck("check_files", []string{"paths=noneex"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state Unknown")
	assert.Contains(t, string(res.BuildPluginOutput()), "UNKNOWN - noneex: no such file or directory")

	res = snc.RunCheck("check_files", []string{"path=.", "filter=name eq 'check_files.go' and size gt 5K", "crit=count>0", "ok=count eq 0", "empty-state=ok"})
	assert.Equalf(t, CheckExitCritical, res.State, "state Critical")
	assert.Contains(t, string(res.BuildPluginOutput()), "'count'=")

	res = snc.RunCheck("check_files", []string{"paths=t", "crit=size>10M"})
	assert.Contains(t, string(res.BuildPluginOutput()), ";10000000;")

	res = snc.RunCheck("check_files", []string{"paths=t", "crit=size>10m"})
	assert.Contains(t, string(res.BuildPluginOutput()), ";10000000;")

	res = snc.RunCheck("check_files", []string{"paths=t", "crit=size>10G"})
	assert.Contains(t, string(res.BuildPluginOutput()), ";10000000000;")

	res = snc.RunCheck("check_files", []string{"paths=t", "crit=size gt 10g"})
	assert.Contains(t, string(res.BuildPluginOutput()), ";10000000000;")

	res = snc.RunCheck("check_files", []string{"paths=t", "crit=age gt 10m", "show-all"})
	assert.Contains(t, string(res.BuildPluginOutput()), ";600;")

	res = snc.RunCheck("check_files", []string{"paths=t", "crit=written lt -10m", "show-all"})
	output := string(res.BuildPluginOutput())
	minus10Min := time.Now().Unix() - 600
	// allow 3seconds time gap to avoid false negatives
	for range 3 {
		if !strings.Contains(output, fmt.Sprintf(";%d:;", minus10Min)) {
			minus10Min--
		}
	}
	assert.Contains(t, output, fmt.Sprintf(";%d:;", minus10Min))

	res = snc.RunCheck("check_files", []string{"path=./t/checksum.txt", "crit=md5_checksum == 3687C5D7106484CD61CDE867A2A999FA"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "files are ok")

	res = snc.RunCheck("check_files", []string{"path=./t/checksum.txt", "crit=md5_checksum != 3687C5D7106484CD61CDE867A2A999FA"})
	assert.Equalf(t, CheckExitCritical, res.State, "CRITICAL")
	assert.Contains(t, string(res.BuildPluginOutput()), "0/1 files")

	res = snc.RunCheck("check_files", []string{"path=./t/checksum.txt", "crit=sha1_checksum == 4EE4BFE9AA51E56A7BD5CCF4785C35A27EE022F8"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "files are ok")

	res = snc.RunCheck("check_files", []string{"path=./t/checksum.txt", "crit=sha256_checksum == 4BCF93F8BA02358F5F48FFF38F5FF0B766284AC319D76A83A471D1C811DF1341"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "files are ok")

	res = snc.RunCheck("check_files", []string{"path=./t/checksum.txt", "crit=sha384_checksum == 5E3751ECD7A74B7B2D98387EAD2F5EA6563BDACDC3F34E3177DD9823B55AF959532148403CC060EE5F872F4BD8E8492A"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "files are ok")

	res = snc.RunCheck("check_files", []string{
		"path=./t/checksum.txt",
		"crit=sha512_checksum == 5D2A522D766BE977445451C07B7394F9EF0E4CA091FFD8866E3FF2AD7F83D67F5CA6B9BD37CDFFB9E338A426CD18D56DFD57C42FF2255B193FB20811F5F5EA80",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "files are ok")

	StopTestAgent(t, snc)
}

func TestCheckFilesNoPermission(t *testing.T) {
	snc := StartTestAgent(t, "")
	// prepare test folder
	tmpPath := t.TempDir()

	for _, char := range []string{"a", "b", "c"} {
		err := os.WriteFile(filepath.Join(tmpPath, "file "+char+".txt"), []byte(strings.Repeat(char, 2000)), 0o600)
		require.NoErrorf(t, err, "writing worked")

		err = os.Mkdir(filepath.Join(tmpPath, "dir "+char), 0o700)
		require.NoErrorf(t, err, "writing worked")
	}
	err := os.Chmod(filepath.Join(tmpPath, "file b.txt"), 0o000)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(tmpPath, "dir b"), 0o000)
	require.NoError(t, err)

	res := snc.RunCheck("check_files", []string{"path=" + tmpPath, "filter=name eq 'file a.txt' and size gt 1K", "crit=count>0", "ok=count eq 0", "empty-state=ok"})
	assert.Equalf(t, CheckExitCritical, res.State, "state Critical")
	assert.Contains(t, string(res.BuildPluginOutput()), "'count'=")

	res = snc.RunCheck("check_files", []string{"path=" + tmpPath})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Contains(t, string(res.BuildPluginOutput()), "OK - All 6 files are ok")

	defer os.RemoveAll(tmpPath)

	StopTestAgent(t, snc)
}

func TestCheckFilesFilterDateKeywords(t *testing.T) {
	snc := StartTestAgent(t, "")
	tmpPath := t.TempDir()

	prepareDateKeywordFiles(t, tmpPath, false)

	tests := []struct {
		keyword string
		older   string
		newer   string
	}{
		{"today", "yesterday.txt", "today.txt"},
		{"thisweek", "lastweek.txt", "thisweek.txt"},
		{"thismonth", "lastmonth.txt", "thismonth.txt"},
		{"thisyear", "lastyear.txt", "thisyear.txt"},
	}

	for _, tc := range tests {
		verifyCheckFilesDateKeyword(t, snc, tmpPath, tc.keyword, tc.older, tc.newer)
	}

	StopTestAgent(t, snc)
}

func TestCheckFilesFilterDateKeywordsUTC(t *testing.T) {
	snc := StartTestAgent(t, "")
	tmpPath := t.TempDir()

	prepareDateKeywordFiles(t, tmpPath, true)

	tests := []struct {
		keyword string
		older   string
		newer   string
	}{
		{"today:utc", "yesterday_utc.txt", "today_utc.txt"},
		{"thisweek:utc", "lastweek_utc.txt", "thisweek_utc.txt"},
		{"thismonth:utc", "lastmonth_utc.txt", "thismonth_utc.txt"},
		{"thisyear:utc", "lastyear_utc.txt", "thisyear_utc.txt"},
	}

	for _, tc := range tests {
		verifyCheckFilesDateKeyword(t, snc, tmpPath, tc.keyword, tc.older, tc.newer)
	}

	StopTestAgent(t, snc)
}

func prepareDateKeywordFiles(t *testing.T, tmpPath string, utc bool) {
	t.Helper()
	suffix := ".txt"
	if utc {
		suffix = "_utc.txt"
	}

	now := time.Now()
	if utc {
		now = now.UTC()
	}
	location := now.Location()

	// 1. Prepare files for today vs yesterday
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	yesterday := midnight.Add(-1 * time.Second)
	today := midnight.Add(1 * time.Second)
	createTestFile(t, tmpPath, "yesterday"+suffix, yesterday)
	createTestFile(t, tmpPath, "today"+suffix, today)

	// 2. Prepare files for thisweek vs lastweek
	offset := (int(now.Weekday()) + 6) % 7
	startOfWeek := now.AddDate(0, 0, -offset)
	startOfWeekMidnight := time.Date(startOfWeek.Year(), startOfWeek.Month(), startOfWeek.Day(), 0, 0, 0, 0, location)
	lastWeek := startOfWeekMidnight.Add(-1 * time.Second)
	thisWeek := startOfWeekMidnight.Add(1 * time.Second)
	createTestFile(t, tmpPath, "lastweek"+suffix, lastWeek)
	createTestFile(t, tmpPath, "thisweek"+suffix, thisWeek)

	// 3. Prepare files for thismonth vs lastmonth
	startOfMonthMidnight := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
	lastMonth := startOfMonthMidnight.Add(-1 * time.Second)
	thisMonth := startOfMonthMidnight.Add(1 * time.Second)
	createTestFile(t, tmpPath, "lastmonth"+suffix, lastMonth)
	createTestFile(t, tmpPath, "thismonth"+suffix, thisMonth)

	// 4. Prepare files for thisyear vs lastyear
	startOfYearMidnight := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, location)
	lastYear := startOfYearMidnight.Add(-1 * time.Second)
	thisYear := startOfYearMidnight.Add(1 * time.Second)
	createTestFile(t, tmpPath, "lastyear"+suffix, lastYear)
	createTestFile(t, tmpPath, "thisyear"+suffix, thisYear)
}

func createTestFile(t *testing.T, dir, name string, modTime time.Time) {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(name), 0o600))
	require.NoError(t, os.Chtimes(p, modTime, modTime))
}

func verifyCheckFilesDateKeyword(t *testing.T, snc *Agent, tmpPath, keyword, older, newer string) {
	t.Helper()
	t.Run(keyword, func(t *testing.T) {
		// Test: filter=written < keyword (should contain older, not newer)
		res := snc.RunCheck("check_files", []string{
			"path=" + tmpPath,
			"filter=written < " + keyword,
			"crit=count > 0",
			"top-syntax=%(list)",
			"detail-syntax=%(filename)",
		})
		assert.Equalf(t, CheckExitCritical, res.State, "written < %s state Critical", keyword)
		assert.Containsf(t, string(res.BuildPluginOutput()), older, "written < %s contains older", keyword)
		assert.NotContainsf(t, string(res.BuildPluginOutput()), newer, "written < %s not contains newer", keyword)

		// Test: filter=written >= keyword (should contain newer, not older)
		res = snc.RunCheck("check_files", []string{
			"path=" + tmpPath,
			"filter=written >= " + keyword,
			"crit=count > 0",
			"top-syntax=%(list)",
			"detail-syntax=%(filename)",
		})
		assert.Equalf(t, CheckExitCritical, res.State, "written >= %s state Critical", keyword)
		assert.Containsf(t, string(res.BuildPluginOutput()), newer, "written >= %s contains newer", keyword)
		assert.NotContainsf(t, string(res.BuildPluginOutput()), older, "written >= %s not contains older", keyword)
	})
}

// Generates a config file, where snclient can call a script. It registers scriptName as a command.
// scriptName does not have an extension
// scriptFilename does have (most likely an OS specific) script extension.
func checkFilesTestConfigWithScript(t *testing.T, scriptsDir, scriptName, scriptFilename string) string {
	t.Helper()

	configTemplate := `
[/modules]
CheckExternalScripts = enabled

[/paths]
scripts = ${SCRIPTS_DIR}
shared-path = %(scripts)

[/settings/external scripts]
timeout = 1111111
allow arguments = true

[/settings/external scripts/scripts]
${SCRIPT_NAME} = ./${SCRIPT_FILENAME} "$ARG1$"

[/settings/external scripts/scripts/${SCRIPT_NAME}]
allow arguments = true
allow nasty characters = true
`

	mapper := func(placeholderName string) string {
		switch placeholderName {
		case "SCRIPTS_DIR":
			return scriptsDir
		case "SCRIPT_NAME":
			return scriptName
		case "SCRIPT_FILENAME":
			return scriptFilename
		default:
			// if its not some value we know, leave it as is
			return "$" + placeholderName
		}
	}

	return os.Expand(configTemplate, mapper)
}

func TestCheckFilesRecursiveArguments(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")
	scriptName := "check_files_recursive_generate_files"
	scriptFilename := scriptName

	switch runtime.GOOS {
	case "windows":
		scriptFilename += ".ps1"
	case "linux":
		scriptFilename += ".sh"
	default:
		t.Skipf("Test is not intended to be run on %s", runtime.GOOS)
	}

	config := checkFilesTestConfigWithScript(t, scriptsDir, scriptName, scriptFilename)
	snc := StartTestAgent(t, config)

	// capture this since t.TempDir() is dyanic
	geneartionDirectory := t.TempDir()

	res := snc.RunCheck(scriptName, []string{geneartionDirectory})
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
	scriptName := "check_files_perfdata_generate_files"
	scriptFilename := scriptName

	switch runtime.GOOS {
	case "windows":
		scriptFilename += ".ps1"
	case "linux":
		scriptFilename += ".sh"
	case "darwin":
		t.Skip("Skipping on darwin as its 'date' command does not work in the script")
	default:
		t.Skipf("Test is not intended to be run on %s", runtime.GOOS)
	}

	config := checkFilesTestConfigWithScript(t, scriptsDir, scriptName, scriptFilename)
	snc := StartTestAgent(t, config)

	// capture this since t.TempDir() is dyanic
	geneartionDirectory := t.TempDir()

	res := snc.RunCheck("check_files_perfdata_generate_files", []string{geneartionDirectory})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	outputString := string(res.BuildPluginOutput())
	assert.Containsf(t, outputString, "ok - Generated 16 files for testing", "output matches")
	assert.Containsf(t, outputString, "ok - Generated 2 directories for testing", "output matches")

	// check_files_perfdata_generate_files script generates a structure that looks like this
	// The script should build a file tree that looks like this
	// .
	// ├── a
	// │   ├── file_1024kb_1.a
	// │   ├── file_1024kb_2.a
	// │   ├── file_1024kb_3.a
	// │   └── file_1024kb_4.a
	// ├── b
	// │   ├── file_1024kb_1.b
	// │   ├── file_1024kb_2.b
	// │   ├── file_1024kb_3.b
	// │   ├── file_1024kb_4.b
	// │   └── file_1024kb_5.b
	// ├── file_1024kb_1.root
	// ├── file_1024kb_2.root
	// ├── file_1024kb_3.root
	// ├── file_512kb_1.root
	// ├── file_512kb_2.root
	// ├── file_512kb_3.root
	// └── file_512kb_4.root
	//
	// 3 directories, 16 files

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
	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "pattern=*.b", "crit='size > 100MiB'"})
	outputString = string(res.BuildPluginOutput())
	assert.Containsf(t, outputString, "OK - All 6 files are ok", "output matches")
	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "pattern=*.b", "crit='size > 100MiB'", "filter=' type == file'"})
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
