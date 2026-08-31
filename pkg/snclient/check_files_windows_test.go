package snclient

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestCheckPathSpecifications(t *testing.T) {
	snc := StartTestAgent(t, "")
	var res *CheckResult

	// ===============  C:\Windows\Fonts\arial.ttf
	// Intended separator is backward slashes like this
	res = snc.RunCheck("check_files", []string{"path=C:\\Windows\\fonts", "max-depth=1", "filter= type == 'file' and name == 'arial.ttf' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 1 files are ok", "output matches")

	// Forward slashes should be converted to backward slashes
	res = snc.RunCheck("check_files", []string{"path=C:/Windows/fonts", "max-depth=1", "filter= type == 'file' and name == 'arial.ttf' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 1 files are ok", "output matches")

	// Multiple backward slashes that do not actually go into subfolders and add depth should be ignored
	res = snc.RunCheck("check_files", []string{"path=C:\\\\\\Windows\\\\fonts", "max-depth=1", "filter= type == 'file' and name == 'arial.ttf' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 1 files are ok", "output matches")

	// Multiple backward slashes that do not actually go into subfolders and add depth should be ignored
	res = snc.RunCheck("check_files", []string{"path=C:\\\\Windows\\\\fonts", "max-depth=1", "filter= type == 'file' and name == 'arial.ttf' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 1 files are ok", "output matches")

	// Multiple forward slashes that do not actually go into subfolders and add depth should be ignored
	res = snc.RunCheck("check_files", []string{"path=C://Windows////fonts", "max-depth=1", "filter= type == 'file' and name == 'arial.ttf' "})
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 1 files are ok", "output matches")

	StopTestAgent(t, snc)
}

// TestCheckFilesDiskSizeWindows verifies disksize matches the OS-reported allocation size
// (Explorer "Size on disk") for a small file and a directory.
func TestCheckFilesDiskSizeWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows specific")
	}
	snc := StartTestAgent(t, "")
	dir := t.TempDir()

	// 1-byte file: disksize == allocated cluster size (>= logical size)
	small := filepath.Join(dir, "small.bin")
	require.NoError(t, os.WriteFile(small, []byte{1}, 0o600))

	res := snc.RunCheck("check_files", []string{"path=" + dir, "add-disk-size=true", "crit='disksize < 0'", "filter='type == file'"})
	require.Equalf(t, CheckExitOK, res.State, "state OK")
	output := string(res.BuildPluginOutput())

	// the reported disksize must equal the OS allocation size (Explorer "Size on disk")
	info, err := os.Stat(small)
	require.NoError(t, err)
	wantDiskSize, err := getFileDiskSize(info, small)
	require.NoError(t, err)
	assert.Containsf(t, output, fmt.Sprintf("'small.bin disksize'=%dB", wantDiskSize),
		"disksize matches OS allocation size (want %d)", wantDiskSize)
	assert.GreaterOrEqualf(t, wantDiskSize, uint64(1), "a 1-byte file allocates at least one cluster")

	// a directory entry reports its own allocated size (not the sum of its contents)
	res = snc.RunCheck("check_files", []string{"path=" + dir, "add-disk-size=true", "crit='disksize < 0'", "filter='type == dir'"})
	require.Equalf(t, CheckExitOK, res.State, "state OK")
	output = string(res.BuildPluginOutput())
	re := regexp.MustCompile(`'.+ disksize'=(\d+)B`)
	m := re.FindStringSubmatch(output)
	require.Lenf(t, m, 2, "directory disksize metric missing: %s", output)
	dirDiskSize, err := strconv.ParseUint(m[1], 10, 64)
	require.NoError(t, err)
	// the directory's own allocation is far smaller than the file's contents would suggest;
	// assert it is a positive, small value
	assert.Positivef(t, dirDiskSize, "directory has an allocated size")
	assert.Lessf(t, dirDiskSize, uint64(1<<20), "directory entry allocation is small (< 1 MiB)")

	StopTestAgent(t, snc)
}
