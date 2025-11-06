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
	assert.Equalf(t, CheckExitUnknown, res.State, "state Unknown")
	assert.Contains(t, string(res.BuildPluginOutput()), "No files found")

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

func check_files_configfile(t *testing.T, scriptsDir, scriptsType string) string {
	t.Helper()

	config := fmt.Sprintf(`
[/modules]
CheckExternalScripts = enabled

[/paths]
scripts = %s
shared-path = %%(scripts)

[/settings/external scripts/wrappings]
sh = %%SCRIPT%% %%ARGS%%
exe = %%SCRIPT%% %%ARGS%%

[/settings/external scripts]
timeout = 1111111
allow arguments = true

[/settings/external scripts/scripts]
check_files_generate_files = ./check_files_generate_files.EXTENSION "$ARG1$"
`, scriptsDir)

	config = strings.ReplaceAll(config, "EXTENSION", scriptsType)

	return config
}

func TestTimeKeywordFilters(t *testing.T) {

	// prepare a tempdir
	tempDir := t.TempDir()

	fmt.Printf("tempDir= %s \n", tempDir)

	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")

	config := check_files_configfile(t, scriptsDir, "sh")
	snc := StartTestAgent(t, config)

	// There is a bash script on this path: pkg/snclient/t/scripts/check_files_generate_files.sh
	// It generates files on a temporary path, and changes their modification date
	// This script is added to the snclient config first and registered as a check command, then ran by the snclient executable itself
	res := snc.RunCheck("check_files_generate_files", []string{tempDir})

	// one_year_from_now_on
	// one_month_from_now_on
	// one_week_from_now_on
	// two_days_from_now_on
	// tomorrow
	// today
	// yesterday
	// two_days_ago
	// one_week_ago
	// one_month_ago
	// one_year_ago
	assert.Equalf(t, CheckExitOK, res.State, "Generating test files successfull")
	assert.Equalf(t, string(res.BuildPluginOutput()), "ok - Generated 11 files for testing", "output matches")

	// Note on 2025-11-06 : Multiple filter="<condition>"s are combined with a logical OR.
	// res = snc.RunCheck("check_files", []string{fmt.Sprintf("path=%s", tempDir), "filter=\"written>=today\"", "filter=\"written<tomorrow\""})
	// Such a test got every file

	res = snc.RunCheck("check_files", []string{fmt.Sprintf("path=%s", tempDir), "filter=\"written=today\""})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 1 files are ok", "output matches")

	res = snc.RunCheck("check_files", []string{fmt.Sprintf("path=%s", tempDir), "filter=\"written>today\""})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 5 files are ok", "output matches")

	StopTestAgent(t, snc)
}
