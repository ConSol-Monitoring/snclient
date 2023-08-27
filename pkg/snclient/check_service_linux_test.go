package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckServiceLinux(t *testing.T) {
	flags := &AgentFlags{
		Quiet: true,
	}
	snc := NewAgent(flags)

	initSet, _ := snc.Init()
	snc.initSet = initSet
	snc.Tasks = initSet.tasks
	snc.Config = initSet.config

	res := snc.RunCheck("check_service", []string{"filter='state=running'"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		regexp.MustCompile(`^OK: All \d+ service\(s\) are ok.`),
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_service", []string{"service=nonexistingservice"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state Unknown")
	assert.Containsf(t, string(res.BuildPluginOutput()), "UNKNOWN - could not find service: nonexistingservice", "output matches")
}

func TestCheckServiceLinuxSystemCtlOutput_1(t *testing.T) {
	output := `● blah-service.service
	Loaded: loaded (/usr/lib/blub/blah-service.sh; enabled; preset: enabled)
	Active: active (exited) since Thu 2023-06-29 08:12:45 CEST; 1 day 3h ago
	   CPU: 12ms

Jun 29 08:12:45 host systemd[1]: Starting blah-service.service...
Jun 29 08:12:45 host systemd[1]: Started blah-service.service.
`

	cs := &CheckService{}
	entry := cs.parseSystemCtlStatus("blah", output)

	expect := map[string]string{
		"name":    "blah",
		"service": "blah",
		"desc":    "",
		"state":   "oneshot",
		"preset":  "enabled",
		"created": "Thu 2023-06-29 08:12:45 CEST",
		"pid":     "",
		"mem":     "",
	}
	assert.Equalf(t, expect, entry, "parsed systemctl output")
}

func TestCheckServiceLinuxSystemCtlOutput_2(t *testing.T) {
	output := `● uuidd.service - Daemon for generating UUIDs
	Loaded: loaded (/lib/systemd/system/uuidd.service; indirect; preset: enabled)
	Active: active (running) since Thu 2023-06-29 08:24:22 CEST; 1 day 4h ago
TriggeredBy: ● uuidd.socket
	  Docs: man:uuidd(8)
  Main PID: 15288 (uuidd)
	 Tasks: 1 (limit: 18801)
	Memory: 540.0K
	   CPU: 106ms
	CGroup: /system.slice/uuidd.service
			└─15288 /usr/sbin/uuidd --socket-activation

Jun 29 08:24:22 host systemd[1]: Started uuidd.service - Daemon for generating UUIDs.
`

	cs := &CheckService{}
	entry := cs.parseSystemCtlStatus("uuidd", output)

	expect := map[string]string{
		"name":    "uuidd",
		"service": "uuidd",
		"desc":    "Daemon for generating UUIDs",
		"state":   "running",
		"preset":  "enabled",
		"created": "Thu 2023-06-29 08:24:22 CEST",
		"pid":     "15288",
		"mem":     "540.0K",
	}
	assert.Equalf(t, expect, entry, "parsed systemctl output")
}

func TestCheckServiceLinuxSystemCtlOutput_3(t *testing.T) {
	output := `× openipmi.service - LSB: OpenIPMI Driver init script
	Loaded: loaded (/etc/init.d/openipmi; generated)
	Active: failed (Result: exit-code) since Thu 2023-06-29 08:12:48 CEST; 1 day 4h ago
	  Docs: man:systemd-sysv-generator(8)
	   CPU: 77ms

Jun 29 08:12:48 host systemd[1]: Starting openipmi.service - LSB: OpenIPMI Driver init script...
Jun 29 08:12:48 host openipmi[1914]: Starting ipmi drivers ipmi failed!
Jun 29 08:12:48 host openipmi[1914]: .
Jun 29 08:12:48 host systemd[1]: openipmi.service: Control process exited, code=exited, status=1/FAILURE
Jun 29 08:12:48 host systemd[1]: openipmi.service: Failed with result 'exit-code'.
Jun 29 08:12:48 host systemd[1]: Failed to start openipmi.service - LSB: OpenIPMI Driver init script.
`

	cs := &CheckService{}
	entry := cs.parseSystemCtlStatus("openipmi", output)

	expect := map[string]string{
		"name":    "openipmi",
		"service": "openipmi",
		"desc":    "LSB: OpenIPMI Driver init script",
		"state":   "stopped",
		"preset":  "",
		"created": "Thu 2023-06-29 08:12:48 CEST",
		"pid":     "",
		"mem":     "",
	}
	assert.Equalf(t, expect, entry, "parsed systemctl output")
}

func TestCheckServiceLinuxSystemCtlOutput_4(t *testing.T) {
	output := `● postfix.service - Postfix Mail Transport Agent
	Loaded: loaded (/usr/lib/systemd/system/postfix.service; enabled; vendor preset: disabled)
	Active: active (running) since Fri 2023-06-30 10:14:21 CEST; 2h 37min ago
   Process: 1048 ExecStart=/usr/sbin/postfix start (code=exited, status=0/SUCCESS)
   Process: 1035 ExecStartPre=/usr/libexec/postfix/chroot-update (code=exited, status=0/SUCCESS)
   Process: 1016 ExecStartPre=/usr/libexec/postfix/aliasesdb (code=exited, status=0/SUCCESS)
  Main PID: 1409 (master)
	CGroup: /system.slice/postfix.service
			├─ 1409 /usr/libexec/postfix/master -w
			├─ 1421 qmgr -l -t unix -u
			└─18683 pickup -l -t unix -u

 Jun 30 10:14:20 centos7-64 systemd[1]: Starting Postfix Mail Transport Agent...
 Jun 30 10:14:21 centos7-64 postfix/master[1409]: daemon started -- version 2.10.1, configuration /etc/postfix
 Jun 30 10:14:21 centos7-64 systemd[1]: Started Postfix Mail Transport Agent.
 `

	cs := &CheckService{}
	entry := cs.parseSystemCtlStatus("postfix", output)

	expect := map[string]string{
		"name":    "postfix",
		"service": "postfix",
		"desc":    "Postfix Mail Transport Agent",
		"state":   "running",
		"preset":  "disabled",
		"created": "Fri 2023-06-30 10:14:21 CEST",
		"pid":     "1409",
		"mem":     "",
	}
	assert.Equalf(t, expect, entry, "parsed systemctl output")
}

func TestCheckServiceLinuxSystemCtlOutput_5(t *testing.T) {
	output := `○ systemd-fsck@dev-disk-byx2duuid-A811x2d4B23.service - File System Check on /dev/disk/byx2duuid/A811x2d4B23
	Loaded: loaded (/lib/systemd/system/systemd-fsck@.service; static)
	Active: inactive (dead)
	  Docs: man:systemd-fsck@.service(8)`

	cs := &CheckService{}
	entry := cs.parseSystemCtlStatus("systemd-fsck@dev-disk-by\x2duuid-A811\x2d4B23", output)

	expect := map[string]string{
		"name":    "systemd-fsck@dev-disk-by\x2duuid-A811\x2d4B23",
		"service": "systemd-fsck@dev-disk-by\x2duuid-A811\x2d4B23",
		"desc":    "File System Check on /dev/disk/byx2duuid/A811x2d4B23",
		"state":   "static",
		"preset":  "",
		"created": "",
		"pid":     "",
		"mem":     "",
	}
	assert.Equalf(t, expect, entry, "parsed systemctl output")
}
