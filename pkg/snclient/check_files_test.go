package snclient

import (
	"fmt"
	"os"
	"path/filepath"
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

func TestCheckFilesFilterToday(t *testing.T) {
	snc := StartTestAgent(t, "")
	// prepare test folder
	tmpPath := t.TempDir()

	// create file from yesterday
	yesterday := time.Now().AddDate(0, 0, -1)
	err := os.WriteFile(filepath.Join(tmpPath, "yesterday.txt"), []byte("yesterday"), 0o600)
	require.NoError(t, err)
	err = os.Chtimes(filepath.Join(tmpPath, "yesterday.txt"), yesterday, yesterday)
	require.NoError(t, err)

	// create file from today (early morning)
	now := time.Now()
	todayEarly := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 1, 0, time.Local)
	// if we are running at exactly 00:00:00, this test might be flaky if we don't ensure todayEarly is actually today.
	// But 1 second after midnight should be safe.
	err = os.WriteFile(filepath.Join(tmpPath, "today.txt"), []byte("today"), 0o600)
	require.NoError(t, err)
	err = os.Chtimes(filepath.Join(tmpPath, "today.txt"), todayEarly, todayEarly)
	require.NoError(t, err)

	// Test 1: filter=written < today
	// Should match yesterday.txt, but NOT today.txt
	res := snc.RunCheck("check_files", []string{
		"path=" + tmpPath,
		"filter=written < today",
		"crit=count > 0",
		"top-syntax=%(list)",
		"detail-syntax=%(filename)",
	})
	assert.Equal(t, CheckExitCritical, res.State, "state Critical")
	assert.Contains(t, string(res.BuildPluginOutput()), "yesterday.txt")
	assert.NotContains(t, string(res.BuildPluginOutput()), "today.txt")

	// Test 2: filter=written >= today
	// Should match today.txt, but NOT yesterday.txt
	res = snc.RunCheck("check_files", []string{
		"path=" + tmpPath,
		"filter=written >= today",
		"crit=count > 0",
		"top-syntax=%(list)",
		"detail-syntax=%(filename)",
	})
	assert.Equal(t, CheckExitCritical, res.State, "state Critical")
	assert.Contains(t, string(res.BuildPluginOutput()), "today.txt")
	assert.NotContains(t, string(res.BuildPluginOutput()), "yesterday.txt")

	StopTestAgent(t, snc)
}

func TestCheckFilesFilterTodayUTC(t *testing.T) {
	snc := StartTestAgent(t, "")
	// prepare test folder
	tmpPath := t.TempDir()

	nowUTC := time.Now().UTC()
	midnightUTC := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)

	// create file before UTC midnight
	yesterdayUTC := midnightUTC.Add(-1 * time.Second)
	err := os.WriteFile(filepath.Join(tmpPath, "yesterday_utc.txt"), []byte("yesterday_utc"), 0o600)
	require.NoError(t, err)
	err = os.Chtimes(filepath.Join(tmpPath, "yesterday_utc.txt"), yesterdayUTC, yesterdayUTC)
	require.NoError(t, err)

	// create file after UTC midnight
	todayUTC := midnightUTC.Add(1 * time.Second)
	err = os.WriteFile(filepath.Join(tmpPath, "today_utc.txt"), []byte("today_utc"), 0o600)
	require.NoError(t, err)
	err = os.Chtimes(filepath.Join(tmpPath, "today_utc.txt"), todayUTC, todayUTC)
	require.NoError(t, err)

	// Test 1: filter=written < today:utc
	res := snc.RunCheck("check_files", []string{
		"path=" + tmpPath,
		"filter=written < today:utc",
		"crit=count > 0",
		"top-syntax=%(list)",
		"detail-syntax=%(filename)",
	})
	assert.Equal(t, CheckExitCritical, res.State, "state Critical")
	assert.Contains(t, string(res.BuildPluginOutput()), "yesterday_utc.txt")
	assert.NotContains(t, string(res.BuildPluginOutput()), "today_utc.txt")

	// Test 2: filter=written >= today:utc
	res = snc.RunCheck("check_files", []string{
		"path=" + tmpPath,
		"filter=written >= today:utc",
		"crit=count > 0",
		"top-syntax=%(list)",
		"detail-syntax=%(filename)",
	})
	assert.Equal(t, CheckExitCritical, res.State, "state Critical")
	assert.Contains(t, string(res.BuildPluginOutput()), "today_utc.txt")
	assert.NotContains(t, string(res.BuildPluginOutput()), "yesterday_utc.txt")

	StopTestAgent(t, snc)
}
