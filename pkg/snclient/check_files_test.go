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

func TestCheckFilesFilterDateKeywords(t *testing.T) {
	snc := StartTestAgent(t, "")
	tmpPath := t.TempDir()

	// 1. Prepare files for today vs yesterday
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := midnight.Add(-1 * time.Second)
	today := midnight.Add(1 * time.Second)
	require.NoError(t, os.WriteFile(filepath.Join(tmpPath, "yesterday.txt"), []byte("yesterday"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(tmpPath, "yesterday.txt"), yesterday, yesterday))
	require.NoError(t, os.WriteFile(filepath.Join(tmpPath, "today.txt"), []byte("today"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(tmpPath, "today.txt"), today, today))

	// 2. Prepare files for thisweek vs lastweek
	// Monday = 1 ... Sunday = 7 (in Go Sunday is 0)
	// We want offset from Monday. 0=Monday, ..., 6=Sunday
	offset := (int(now.Weekday()) + 6) % 7
	startOfWeek := now.AddDate(0, 0, -offset)
	startOfWeekMidnight := time.Date(startOfWeek.Year(), startOfWeek.Month(), startOfWeek.Day(), 0, 0, 0, 0, now.Location())
	lastWeek := startOfWeekMidnight.Add(-1 * time.Second)
	thisWeek := startOfWeekMidnight.Add(1 * time.Second)
	require.NoError(t, os.WriteFile(filepath.Join(tmpPath, "lastweek.txt"), []byte("lastweek"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(tmpPath, "lastweek.txt"), lastWeek, lastWeek))
	require.NoError(t, os.WriteFile(filepath.Join(tmpPath, "thisweek.txt"), []byte("thisweek"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(tmpPath, "thisweek.txt"), thisWeek, thisWeek))

	// 3. Prepare files for thismonth vs lastmonth
	startOfMonthMidnight := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastMonth := startOfMonthMidnight.Add(-1 * time.Second)
	thisMonth := startOfMonthMidnight.Add(1 * time.Second)
	require.NoError(t, os.WriteFile(filepath.Join(tmpPath, "lastmonth.txt"), []byte("lastmonth"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(tmpPath, "lastmonth.txt"), lastMonth, lastMonth))
	require.NoError(t, os.WriteFile(filepath.Join(tmpPath, "thismonth.txt"), []byte("thismonth"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(tmpPath, "thismonth.txt"), thisMonth, thisMonth))

	// 4. Prepare files for thisyear vs lastyear
	startOfYearMidnight := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, now.Location())
	lastYear := startOfYearMidnight.Add(-1 * time.Second)
	thisYear := startOfYearMidnight.Add(1 * time.Second)
	require.NoError(t, os.WriteFile(filepath.Join(tmpPath, "lastyear.txt"), []byte("lastyear"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(tmpPath, "lastyear.txt"), lastYear, lastYear))
	require.NoError(t, os.WriteFile(filepath.Join(tmpPath, "thisyear.txt"), []byte("thisyear"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(tmpPath, "thisyear.txt"), thisYear, thisYear))

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

	// 1. Prepare files for today:utc vs yesterday
	now := time.Now().UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	yesterday := midnight.Add(-1 * time.Second)
	today := midnight.Add(1 * time.Second)
	require.NoError(t, os.WriteFile(filepath.Join(tmpPath, "yesterday_utc.txt"), []byte("yesterday_utc"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(tmpPath, "yesterday_utc.txt"), yesterday, yesterday))
	require.NoError(t, os.WriteFile(filepath.Join(tmpPath, "today_utc.txt"), []byte("today_utc"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(tmpPath, "today_utc.txt"), today, today))

	// 2. Prepare files for thisweek:utc vs lastweek
	offset := (int(now.Weekday()) + 6) % 7
	startOfWeek := now.AddDate(0, 0, -offset)
	startOfWeekMidnight := time.Date(startOfWeek.Year(), startOfWeek.Month(), startOfWeek.Day(), 0, 0, 0, 0, time.UTC)
	lastWeek := startOfWeekMidnight.Add(-1 * time.Second)
	thisWeek := startOfWeekMidnight.Add(1 * time.Second)
	require.NoError(t, os.WriteFile(filepath.Join(tmpPath, "lastweek_utc.txt"), []byte("lastweek_utc"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(tmpPath, "lastweek_utc.txt"), lastWeek, lastWeek))
	require.NoError(t, os.WriteFile(filepath.Join(tmpPath, "thisweek_utc.txt"), []byte("thisweek_utc"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(tmpPath, "thisweek_utc.txt"), thisWeek, thisWeek))

	// 3. Prepare files for thismonth:utc vs lastmonth
	startOfMonthMidnight := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	lastMonth := startOfMonthMidnight.Add(-1 * time.Second)
	thisMonth := startOfMonthMidnight.Add(1 * time.Second)
	require.NoError(t, os.WriteFile(filepath.Join(tmpPath, "lastmonth_utc.txt"), []byte("lastmonth_utc"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(tmpPath, "lastmonth_utc.txt"), lastMonth, lastMonth))
	require.NoError(t, os.WriteFile(filepath.Join(tmpPath, "thismonth_utc.txt"), []byte("thismonth_utc"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(tmpPath, "thismonth_utc.txt"), thisMonth, thisMonth))

	// 4. Prepare files for thisyear:utc vs lastyear
	startOfYearMidnight := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	lastYear := startOfYearMidnight.Add(-1 * time.Second)
	thisYear := startOfYearMidnight.Add(1 * time.Second)
	require.NoError(t, os.WriteFile(filepath.Join(tmpPath, "lastyear_utc.txt"), []byte("lastyear_utc"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(tmpPath, "lastyear_utc.txt"), lastYear, lastYear))
	require.NoError(t, os.WriteFile(filepath.Join(tmpPath, "thisyear_utc.txt"), []byte("thisyear_utc"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(tmpPath, "thisyear_utc.txt"), thisYear, thisYear))

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
