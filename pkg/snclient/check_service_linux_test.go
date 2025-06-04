package snclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckServiceLinux(t *testing.T) {
	flags := &AgentFlags{
		Quiet: true,
	}
	snc := NewAgent(flags)

	initSet, _ := snc.Init()
	snc.runSet = initSet
	snc.Tasks = initSet.tasks
	snc.config = initSet.config

	res := snc.RunCheck("check_service", []string{"filter='state=running'"})
	assert.Regexpf(t,
		`^OK - All \d+ service\(s\) are ok.|UNKNOWN - No services found`,
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
		"active":  "active",
		"preset":  "enabled",
		"created": "",
		"age":     "",
		"pid":     "",
		"cpu":     "",
		"rss":     "",
		"vms":     "",
		"tasks":   "",
	}
	assert.Equalf(t, expect, entry, "parsed systemctl output")
}

func TestCheckServiceLinuxSystemCtlOutput_2(t *testing.T) {
	output := `● uuidd.service - Daemon for generating UUIDs
	Loaded: loaded (/lib/systemd/system/uuidd.service; indirect; preset: enabled)
	Active: active (running) since Thu 2023-06-29 08:24:22 CEST; 1 day 4h ago
TriggeredBy: ● uuidd.socket
	  Docs: man:uuidd(8)
  Main PID: 152880000 (uuidd)
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
		"active":  "active",
		"preset":  "enabled",
		"created": "",
		"pid":     "152880000", // fake pid which does not exist
		"age":     "",
		"cpu":     "",
		"rss":     "",
		"vms":     "",
		"tasks":   "1",
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
		"active":  "failed",
		"preset":  "",
		"created": "",
		"age":     "",
		"pid":     "",
		"cpu":     "",
		"rss":     "",
		"vms":     "",
		"tasks":   "",
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
  Main PID: 140900000 (master)
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
		"active":  "active",
		"preset":  "disabled",
		"created": "",
		"pid":     "140900000", // fake pid must not exist, otherwise mem/cpu would be filled
		"age":     "",
		"cpu":     "",
		"rss":     "",
		"vms":     "",
		"tasks":   "",
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
		"active":  "inactive",
		"preset":  "",
		"created": "",
		"pid":     "",
		"age":     "",
		"cpu":     "",
		"rss":     "",
		"vms":     "",
		"tasks":   "",
	}
	assert.Equalf(t, expect, entry, "parsed systemctl output")
}

func TestCheckServiceLinuxSystemCtlOutput_6(t *testing.T) {
	output := `× dnf-makecache.service - dnf makecache
     Loaded: loaded (/usr/lib/systemd/system/dnf-makecache.service; static)
     Active: failed (Result: exit-code) since Wed 2024-11-27 14:39:00 CET; 36min ago
TriggeredBy: ● dnf-makecache.timer
    Process: 4033387 ExecStart=/usr/bin/dnf makecache --timer (code=exited, status=1/FAILURE)
   Main PID: 4033387 (code=exited, status=1/FAILURE)
        CPU: 431ms`

	cs := &CheckService{}
	entry := cs.parseSystemCtlStatus("dnf-makecache", output)

	expect := map[string]string{
		"name":    "dnf-makecache",
		"service": "dnf-makecache",
		"desc":    "dnf makecache",
		"state":   "static",
		"active":  "failed",
		"preset":  "",
		"created": "",
		"pid":     "4033387",
		"age":     "",
		"cpu":     "",
		"rss":     "",
		"vms":     "",
		"tasks":   "",
	}
	assert.Equalf(t, expect, entry, "parsed systemctl output")
}

func TestCheckServiceLinuxOutput(t *testing.T) {
	flags := &AgentFlags{
		Quiet: true,
	}
	snc := NewAgent(flags)

	initSet, _ := snc.Init()
	snc.runSet = initSet
	snc.Tasks = initSet.tasks
	snc.config = initSet.config

	// find test service
	inv, err := snc.getInventoryEntry(context.TODO(), "check_service")
	require.NoError(t, err)
	require.NotEmptyf(t, inv, "expected services list to be non-empty")
	var serviceName string
	for _, entry := range inv {
		if entry["state"] != "running" {
			continue
		}
		serviceName = entry["name"]
	}

	require.NotEmptyf(t, serviceName, "check requires a service to test against")

	res := snc.RunCheck("check_service", []string{"filter= name like " + serviceName, "crit=rss<5"})
	assert.Contains(t, string(res.BuildPluginOutput()), "rss")
	assert.Contains(t, string(res.BuildPluginOutput()), "vms")

	res = snc.RunCheck("check_service", []string{"filter= name like " + serviceName, "show-all"})
	assert.Contains(t, string(res.BuildPluginOutput()), "rss")
	assert.Contains(t, string(res.BuildPluginOutput()), "vms")
	assert.Contains(t, string(res.BuildPluginOutput()), "cpu")
	assert.Contains(t, string(res.BuildPluginOutput()), "tasks")

	res = snc.RunCheck("check_service", []string{"service=" + serviceName})
	assert.Contains(t, string(res.BuildPluginOutput()), "rss")
	assert.Contains(t, string(res.BuildPluginOutput()), "vms")
	assert.Contains(t, string(res.BuildPluginOutput()), "cpu")
	assert.Contains(t, string(res.BuildPluginOutput()), "tasks")
}
