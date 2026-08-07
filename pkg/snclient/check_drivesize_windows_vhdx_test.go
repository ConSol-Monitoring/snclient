//go:build windows

package snclient

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

const (
	cmdTimeout           = 2 * time.Minute
	drivesizeVhdxSizeMiB = 10
	maxVhdxSizeBytes     = 50 * 1024 * 1024 // used in check_drivesize assesments
)

func hasElevatedPrivileges() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()

	return token.IsElevated()
}

func runDiskpart(t *testing.T, script string) {
	t.Helper()

	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "diskpart.txt")
	require.NoErrorf(t, os.WriteFile(scriptPath, []byte(script), 0o600), "writing diskpart script")

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "diskpart", "/s", scriptPath).CombinedOutput()
	if err != nil {
		require.NoErrorf(t, err, "diskpart failed: %s\n%s", err.Error(), string(out))
	}
}

// setupDirectoryMountedVolume creates a vhdx volume and mounts it at a directory inside a temp folder.
// the volume is detached again when the test finishes.
// everything is in golang temp test dir, which is deleted once the test ends
func setupDirectoryMountedVolume(t *testing.T, sizeMiB int) string {
	t.Helper()

	tempDir := t.TempDir()
	vhdDir := filepath.Join(tempDir, "vhds")
	mountPath := filepath.Join(tempDir, "testmount", "disk3")
	require.NoErrorf(t, os.MkdirAll(vhdDir, 0o700), "creating VHD directory")
	require.NoErrorf(t, os.MkdirAll(mountPath, 0o700), "creating volume mount directory")

	vhdPath := filepath.Join(vhdDir, "snclient-drivesize-test.vhdx")

	t.Logf("vhdx test: temp test dir: %s", tempDir)
	t.Logf("vhdx test: vhd directory: %s", vhdDir)
	t.Logf("vhdx test: vhd file:      %s", vhdPath)
	t.Logf("vhdx test: mount path:    %s", mountPath)

	t.Cleanup(func() {
		//nolint:gocritic // %q would double-escape the backslashes in the windows path
		runDiskpart(t, fmt.Sprintf("select vdisk file=\"%s\"\ndetach vdisk\n", vhdPath))
		_ = os.RemoveAll(vhdDir)
		_, err := os.Stat(vhdDir)
		vhdDirExists := err == nil
		t.Logf("vhdx test: detached volume, %s still present: %v", vhdDir, vhdDirExists)
	})

	createScript := fmt.Sprintf(`create vdisk file="%s" maximum=%d type=expandable
select vdisk file="%s"
attach vdisk
convert gpt
create partition primary
format fs=ntfs quick label="snclient-test"
assign mount="%s"
`, vhdPath, sizeMiB, vhdPath, mountPath)
	runDiskpart(t, createScript)

	return mountPath
}

// fillVolumeToPercent writes a file into the volume until the used space reaches the target percentage.
// It returns the achieved used percentage, which may be lower if the volume ran full first.
func fillVolumeToPercent(t *testing.T, mountPath string, targetPercent float64) float64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	fillPath := filepath.Join(mountPath, "fill.dat")
	t.Logf("vhdx test: fill file: %s", fillPath)
	file, err := os.OpenFile(fillPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	require.NoErrorf(t, err, "opening fill file")
	defer file.Close()

	// fill with 256 kb chunks
	fillChunkSize := 256 * 1024

	chunk := make([]byte, fillChunkSize)
	usage, err := disk.UsageWithContext(ctx, mountPath)
	require.NoErrorf(t, err, "reading usage of %s", mountPath)
	for usage.UsedPercent < targetPercent {
		if _, writeErr := file.Write(chunk); writeErr != nil {
			break
		}
		usage, err = disk.UsageWithContext(ctx, mountPath)
		require.NoErrorf(t, err, "reading usage of %s", mountPath)
	}

	return usage.UsedPercent
}

// parseSizeUsage reads the size, used and free bytes from a check run that printed
// %(drive_or_name) %(size_bytes) %(used_bytes) %(free_bytes)
// as detail syntax.
func parseSizeUsage(t *testing.T, path, output string) (size, used, free uint64) {
	t.Helper()

	re := regexp.MustCompile(regexp.QuoteMeta(path) + ` (\d+) (\d+) (\d+)`)
	matches := re.FindStringSubmatch(output)
	require.NotNilf(t, matches, "could not find size/used/free for %q in output:\n%s", path, output)

	var err error
	if size, err = strconv.ParseUint(matches[1], 10, 64); err != nil {
		t.Fatalf("parsing size: %v", err)
	}
	if used, err = strconv.ParseUint(matches[2], 10, 64); err != nil {
		t.Fatalf("parsing used: %v", err)
	}
	if free, err = strconv.ParseUint(matches[3], 10, 64); err != nil {
		t.Fatalf("parsing free: %v", err)
	}

	return size, used, free
}

func normalizeVolumePath(path string) string {
	return strings.ToUpper(strings.TrimSuffix(path, "\\"))
}

func TestCheckDrivesizeVolumeMountCustomPathMatching(t *testing.T) {
	if !hasElevatedPrivileges() {
		t.Skipf("creating a vhdx volume requires elevated privileges")
	}

	mountPath := setupDirectoryMountedVolume(t, drivesizeVhdxSizeMiB)

	dummyFolder := filepath.Join(mountPath, "dummy1", "dummy2", "dummy3")
	require.NoErrorf(t, os.MkdirAll(dummyFolder, 0o700), "creating dummy folder inside the mounted volume")
	t.Logf("vhdx test: dummy folder: %s", dummyFolder)

	checkDrivesize := &CheckDrivesize{}
	// the searchPath can directly be the volume mount point or another path inside the volume mount point
	// in both cases, the code should retain the volume mount point, inside _matching_volume_path , and use it when calling GetVolumeInformation
	tests := []struct {
		name       string
		searchPath string
		fallback   bool
		wantDrive  string
	}{
		{"volume root", mountPath, false, mountPath},
		{"folder inside volume", dummyFolder, true, dummyFolder},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			requiredDrives := map[string]map[string]string{}
			err := checkDrivesize.setCustomPath(testCase.searchPath, requiredDrives, testCase.fallback)
			require.NoErrorf(t, err, "setCustomPath(%q) works", testCase.searchPath)

			entry := requiredDrives[testCase.searchPath]
			require.NotNilf(t, entry, "entry exists for %s", testCase.searchPath)

			assert.Equalf(t, testCase.wantDrive, entry["drive"], "drive uses the search path")
			assert.Equalf(t, testCase.wantDrive, entry["drive_or_name"], "drive_or_name uses the search path")
			assert.Equalf(t, testCase.wantDrive, entry["drive_or_id"], "drive_or_id uses the search path")
			assert.Equalf(t, normalizeVolumePath(mountPath), normalizeVolumePath(entry["_matching_volume_path"]),
				"_matching_volume_path retains the volume mount path")
		})
	}
}

func TestCheckDrivesizeVolumeMount(t *testing.T) {
	if !hasElevatedPrivileges() {
		t.Skipf("creating a vhdx volume requires elevated privileges")
	}

	snc := StartTestAgent(t, "")
	defer StopTestAgent(t, snc)

	mountPath := setupDirectoryMountedVolume(t, drivesizeVhdxSizeMiB)

	dummyFolder := filepath.Join(mountPath, "dummy1", "dummy2", "dummy3")
	require.NoErrorf(t, os.MkdirAll(dummyFolder, 0o700), "creating dummy folder inside the mounted volume")
	t.Logf("vhdx test: dummy folder: %s", dummyFolder)

	expectedUsage, err := disk.UsageWithContext(context.Background(), mountPath)
	require.NoErrorf(t, err, "reading expected usage of mounted volume")
	assert.Lessf(t, expectedUsage.Total, uint64(maxVhdxSizeBytes), "mounted volume is the small vhdx, not a real drive")

	// drive argument pointing at the volume mount path
	res := snc.RunCheck("check_drivesize", []string{
		"drive=" + mountPath,
		"warn=used>100%",
		"crit=used>100%",
		"show-all",
	})
	require.Equalf(t, CheckExitOK, res.State, "state OK")
	output := string(res.BuildPluginOutput())
	assert.Containsf(t, output, mountPath, "output contains mount path")
	assert.Containsf(t, output, fmt.Sprintf("'%s used'=", mountPath), "perfdata used label")
	assert.Containsf(t, output, fmt.Sprintf("'%s used %%'=", mountPath), "perfdata used percent label")

	res = snc.RunCheck("check_drivesize", []string{
		"drive=" + mountPath,
		"warn=used>100%",
		"crit=used>100%",
		"show-all",
		"detail-syntax=%(drive_or_name) %(size_bytes) %(used_bytes) %(free_bytes)",
	})
	output = string(res.BuildPluginOutput())
	size, used, free := parseSizeUsage(t, mountPath, output)
	assert.Equalf(t, expectedUsage.Total, size, "reported size matches the mounted volume")
	assert.Equalf(t, used+free, size, "free + used == size")

	// folder argument pointing at a folder inside the mounted volume
	// output and perfdata prefix should use folder path
	res = snc.RunCheck("check_drivesize", []string{
		"folder=" + dummyFolder,
		"warn=used>100%",
		"crit=used>100%",
		"show-all",
	})
	require.Equalf(t, CheckExitOK, res.State, "folder inside mounted volume resolves to the volume")
	output = string(res.BuildPluginOutput())
	assert.Containsf(t, output, dummyFolder, "output contains the folder path")
	assert.Containsf(t, output, fmt.Sprintf("'%s used'=", dummyFolder), "perfdata uses the folder path as label")

	res = snc.RunCheck("check_drivesize", []string{
		"folder=" + dummyFolder,
		"warn=used>100%",
		"crit=used>100%",
		"show-all",
		"detail-syntax=%(drive_or_name) %(size_bytes) %(used_bytes) %(free_bytes)",
	})
	folderSize, _, _ := parseSizeUsage(t, dummyFolder, string(res.BuildPluginOutput()))
	assert.Equalf(t, expectedUsage.Total, folderSize, "folder check reports the volume capacity")

	// all discovery includes the directory mounted volume
	res = snc.RunCheck("check_drivesize", []string{"warn=used>100%", "crit=used>100%", "show-all"})
	require.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Containsf(t, string(res.BuildPluginOutput()), mountPath, "volume is discovered with all")

	// perf-config and perf-syntax like the end-to-end test
	res = snc.RunCheck("check_drivesize", []string{
		"drive=" + mountPath,
		"warn=used>100%",
		"crit=used>100%",
		"perf-config=*(unit:Gb)",
		"perf-syntax=%(key:lc)",
		"show-all",
	})
	require.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Containsf(t, string(res.BuildPluginOutput()), "disk3", "perfdata labels contain the volume name")
}

func TestCheckDrivesizeVolumeMountFull(t *testing.T) {
	if !hasElevatedPrivileges() {
		t.Skipf("creating a vhdx volume requires elevated privileges")
	}

	snc := StartTestAgent(t, "")
	defer StopTestAgent(t, snc)

	mountPath := setupDirectoryMountedVolume(t, drivesizeVhdxSizeMiB)

	// sparse volume is far below the critical threshold at the start
	res := snc.RunCheck("check_drivesize", []string{
		"drive=" + mountPath,
		"warning=none",
		"crit='used gt 90'",
		"show-all",
	})
	require.Equalf(t, CheckExitOK, res.State, "sparse volume is OK")

	// fill the volume up to ~95%
	fillTargetPercent := 95.0
	fillThresholdPercent := 90.0

	achieved := fillVolumeToPercent(t, mountPath, fillTargetPercent)
	require.Greaterf(t, achieved, fillThresholdPercent, "volume was filled beyond the threshold")

	// full volume must trigger the critical threshold
	res = snc.RunCheck("check_drivesize", []string{
		"drive=" + mountPath,
		"warning=none",
		"crit='used gt 90'",
		"show-all",
	})
	require.Equalf(t, CheckExitCritical, res.State, "full volume triggers critical")
	output := string(res.BuildPluginOutput())
	assert.Containsf(t, output, mountPath, "output contains mount path")

	re := regexp.MustCompile(regexp.QuoteMeta(fmt.Sprintf("'%s used %%'=", mountPath)) + `([\d.]+)%`)
	matches := re.FindStringSubmatch(output)
	require.NotNilf(t, matches, "perfdata used percent parsed from output:\n%s", output)
	usedPct, err := strconv.ParseFloat(matches[1], 64)
	require.NoErrorf(t, err, "parsing used percent")
	assert.Greaterf(t, usedPct, fillThresholdPercent, "perfdata used percent exceeds the threshold")

	// warning threshold triggers as well , but not critical which requires 99 percent
	res = snc.RunCheck("check_drivesize", []string{
		"drive=" + mountPath,
		"warning='used gt 90'",
		"crit='used gt 99'",
		"show-all",
	})
	require.Equalf(t, CheckExitWarning, res.State, "full volume triggers warning")
}
