package snclient

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.Equalf(t, "WARNING - 0 security updates / 2 updates available. |'security'=0;;0;0 'updates'=2;0;;0",
		string(res.BuildPluginOutput()), "output matches")

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
	assert.Equalf(t, "CRITICAL - 1 security updates / 3 updates available. |'security'=1;;0;0 'updates'=3;0;;0",
		string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}

func TestCheckYUMUpdates(t *testing.T) {
	snc := StartTestAgent(t, "")

	// mock yum command from output of: yum check-update -q -C
	tmpPath := MockSystemUtilities(t, map[string]string{
		"yum": `

bind-export-libs.x86_64    32:9.11.4-26.P2.el7_9.15       updates
ca-certificates.noarch     2023.2.60_v7.0.306-72.el7_9    updates
cronie.x86_64              1.4.11-25.el7_9                updates
Obsoleting Packages
grub2-tools.x86_64         1:2.06-70.el9_3.2.rocky.0.2    baseos
    grub2-tools.x86_64     1:2.06-46.el9.rocky.0.1        @baseos`,
	})
	defer os.RemoveAll(tmpPath)
	res := snc.RunCheck("check_os_updates", []string{"--system=yum"})
	assert.Equalf(t, CheckExitCritical, res.State, "state Critical")
	assert.Equalf(t, "CRITICAL - 3 security updates / 0 updates available. |'security'=3;;0;0 'updates'=0;0;;0",
		string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}
