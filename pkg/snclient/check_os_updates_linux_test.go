package snclient

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckAPTUpdates(t *testing.T) {
	snc := StartTestAgent(t, "")

	// mock apt-get command from output of: apt-get upgrade
	tmpPath := MockSystemUtilities(t, map[string]string{
		"apt-get": `
NOTE: This is only a simulation!
      apt-get needs root privileges for real execution.
      Keep also in mind that locking is deactivated,
      so don't depend on the relevance to the real current situation!
Inst base-files [12.4+deb12u4] (12.4+deb12u5 Debian:12.5/stable [amd64])
Conf base-files (12.4+deb12u5 Debian:12.5/stable [amd64])
Inst tar [1.34+dfsg-1.2] (1.34+dfsg-1.2+deb12u1 Debian:12.5/stable [amd64])
Conf tar (1.34+dfsg-1.2+deb12u1 Debian:12.5/stable [amd64])`,
	})
	defer os.RemoveAll(tmpPath)
	res := snc.RunCheck("check_os_updates", []string{"--system=apt"})
	assert.Equalf(t, CheckExitWarning, res.State, "state Warning")
	assert.Containsf(t, string(res.BuildPluginOutput()), "WARNING - 0 security updates / 2 updates available. |'security'=0;;0;0 'updates'=2;0;;0", "output matches")

	// mock apt-get command from output of: apt-get upgrade
	tmpPath = MockSystemUtilities(t, map[string]string{
		"apt-get": `
NOTE: This is only a simulation!
      apt-get needs root privileges for real execution.
      Keep also in mind that locking is deactivated,
      so don't depend on the relevance to the real current situation!
Inst base-files [12.4+deb12u4] (12.4+deb12u5 Debian:12.5/stable [amd64])
Conf base-files (12.4+deb12u5 Debian:12.5/stable [amd64])
Inst tar [1.34+dfsg-1.2] (1.34+dfsg-1.2+deb12u1 Debian:12.5/stable [amd64])
Conf tar (1.34+dfsg-1.2+deb12u1 Debian:12.5/stable [amd64])
Inst runc [1.1.5+ds1-1+b1] (1.1.5+ds1-1+deb12u1 Debian:12.5/stable, Debian-Security:12/stable-security [amd64])
Inst steam-libs-i386:i386 [1:1.0.0.78] (1:1.0.0.79 Steam launcher:repo.steampowered.com [i386])`,
	})
	defer os.RemoveAll(tmpPath)
	res = snc.RunCheck("check_os_updates", []string{"--system=apt"})
	assert.Equalf(t, CheckExitCritical, res.State, "state Critical")
	assert.Containsf(t, string(res.BuildPluginOutput()), "CRITICAL - 1 security updates / 3 updates available. |'security'=1;;0;0 'updates'=3;0;;0", "output matches")

	StopTestAgent(t, snc)
}

func TestCheckYUMUpdates(t *testing.T) {
	snc := StartTestAgent(t, "")

	// Mock yum command from output of: yum check-update -q
	yumOutput := `
bind-export-libs.x86_64    32:9.11.4-26.P2.el7_9.15       updates
ca-certificates.noarch     2023.2.60_v7.0.306-72.el7_9    updates
cronie.x86_64              1.4.11-25.el7_9                updates
Obsoleting Packages
grub2-tools.x86_64         1:2.06-70.el9_3.2.rocky.0.2    baseos
    grub2-tools.x86_64     1:2.06-46.el9.rocky.0.1        @baseos`
	argsFile := mockYUMUtility(t, yumOutput, "", 100)
	cacheRoot := filepath.Join(t.TempDir(), "cache root's")
	t.Setenv("CACHE_DIRECTORY", cacheRoot)

	res := snc.RunCheck("check_os_updates", []string{"--system=yum"})
	assert.Equalf(t, CheckExitCritical, res.State, "state Critical")
	assert.Containsf(t, string(res.BuildPluginOutput()), "CRITICAL - 3 security updates / 0 updates available. |'security'=3;;0;0 'updates'=0;0;;0", "output matches")

	argsRaw, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	args := strings.Split(strings.TrimSpace(string(argsRaw)), "\n")
	require.Len(t, args, 2)

	cacheDir := filepath.Join(cacheRoot, "dnf")
	requiredOpts := "<--setopt=cachedir=" + cacheDir + ">" +
		"<--setopt=*.skip_if_unavailable=False>"
	assert.Equal(t, requiredOpts+"<check-update><--security><-q>", args[0])
	assert.Equal(t, requiredOpts+"<check-update><-q>", args[1])
	assert.NotContains(t, string(argsRaw), "<-C>")
	assert.NotContains(t, string(argsRaw), "<--refresh>")

	cacheInfo, err := os.Stat(cacheDir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), cacheInfo.Mode().Perm())

	StopTestAgent(t, snc)
}

func TestCheckYUMUpdateUsesLegacyCompatibleCacheExpiration(t *testing.T) {
	snc := StartTestAgent(t, "")
	argsFile := mockYUMUtility(t, "", "", 0)
	cacheRoot := t.TempDir()
	t.Setenv("CACHE_DIRECTORY", cacheRoot)

	res := snc.RunCheck("check_os_updates", []string{"--system=yum", "--update"})
	assert.Equal(t, CheckExitOK, res.State)

	argsRaw, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	args := strings.Split(strings.TrimSpace(string(argsRaw)), "\n")
	require.Len(t, args, 3)

	cacheDir := filepath.Join(cacheRoot, "dnf")
	requiredOpts := "<--setopt=cachedir=" + cacheDir + ">" +
		"<--setopt=*.skip_if_unavailable=False>"
	assert.Equal(t, requiredOpts+"<clean><expire-cache><-q>", args[0])
	assert.Equal(t, requiredOpts+"<check-update><--security><-q>", args[1])
	assert.Equal(t, requiredOpts+"<check-update><-q>", args[2])
	assert.NotContains(t, string(argsRaw), "<--refresh>")

	StopTestAgent(t, snc)
}

func TestCheckYUMUnavailableRepositoriesFail(t *testing.T) {
	snc := StartTestAgent(t, "")
	mockYUMUtility(t, "", "Failed to download metadata for repo 'baseos'", 1)
	t.Setenv("CACHE_DIRECTORY", t.TempDir())

	res := snc.RunCheck("check_os_updates", []string{"--system=yum"})
	assert.Equal(t, CheckExitUnknown, res.State)
	assert.Contains(t, string(res.BuildPluginOutput()), "yum check-update failed")
	assert.Contains(t, string(res.BuildPluginOutput()), "Failed to download metadata for repo 'baseos'")

	StopTestAgent(t, snc)
}

func TestCheckYUMRejectsSymlinkedCache(t *testing.T) {
	snc := StartTestAgent(t, "")
	mockYUMUtility(t, "", "", 0)
	cacheRoot := t.TempDir()
	t.Setenv("CACHE_DIRECTORY", cacheRoot)
	require.NoError(t, os.Symlink(t.TempDir(), filepath.Join(cacheRoot, "dnf")))

	res := snc.RunCheck("check_os_updates", []string{"--system=yum"})
	assert.Equal(t, CheckExitUnknown, res.State)
	assert.Contains(t, string(res.BuildPluginOutput()), "yum cache path")
	assert.Contains(t, string(res.BuildPluginOutput()), "is not a directory")

	StopTestAgent(t, snc)
}

func mockYUMUtility(t *testing.T, stdout, stderr string, exitCode int) string {
	t.Helper()

	tmpPath := MockSystemUtilities(t, map[string]string{"yum": ""})
	argsFile := filepath.Join(t.TempDir(), "yum-args")
	t.Setenv("YUM_ARGS_FILE", argsFile)

	script := fmt.Sprintf(`#!/bin/sh
for arg do
    printf '<%%s>' "$arg" >> "$YUM_ARGS_FILE"
done
printf '\n' >> "$YUM_ARGS_FILE"
cat <<'YUM_STDOUT'
%s
YUM_STDOUT
cat >&2 <<'YUM_STDERR'
%s
YUM_STDERR
exit %d
`, stdout, stderr, exitCode)
	yumPath := filepath.Join(tmpPath, "yum")
	err := os.WriteFile(yumPath, []byte(script), 0o600)
	require.NoError(t, err)
	err = os.Chmod(yumPath, 0o700)
	require.NoError(t, err)

	return argsFile
}
