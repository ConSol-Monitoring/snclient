package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckPing(t *testing.T) {
	config := `
[/modules]
CheckBuiltinPlugins = enabled

`
	snc := StartTestAgent(t, config)

	res := snc.RunCheck("check_ping", []string{"host=localhost"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")
	assert.Regexpf(t, `^OK - Packet loss = \d+%, RTA = [\d.]+ms \|'rta'=[\d.]+ms;1000;5000 'pl'=\d+%;30;80;0`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_ping", []string{"host=10.99.99.99"})
	assert.Equalf(t, CheckExitCritical, res.State, "state critical")
	assert.Regexpf(t, `^CRITICAL - Packet loss = 100% \|'rta'=U 'pl'=100%;30;80;0`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_ping", []string{"host=should_not_resolve.nowhere"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
	assert.Regexpf(t, `^UNKNOWN - ping: should_not_resolve.nowhere: Name or service not known`, string(res.BuildPluginOutput()), "output matches")

	StopTestAgent(t, snc)
}

func TestPingParserLinuxOK(t *testing.T) {
	exp := map[string]string{
		"host_name": "",
		"sent":      "2",
		"received":  "2",
		"rta":       "0.376",
		"pl":        "0",
		"ttl":       "64",
	}
	// debian 12
	out := `
PING 10.0.1.1 (10.0.1.1) 56(84) bytes of data.
64 bytes from 10.0.1.1: icmp_seq=1 ttl=64 time=0.359 ms
64 bytes from 10.0.1.1: icmp_seq=2 ttl=64 time=0.393 ms

--- 10.0.1.1 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1014ms
rtt min/avg/max/mdev = 0.359/0.376/0.393/0.017 ms
`
	chk := &CheckPing{}
	entry := chk.parsePingOutput(out, "")
	assert.Equalf(t, exp, entry, "parsed ping ok output")
}

func TestPingParserLinuxBad(t *testing.T) {
	exp := map[string]string{
		"host_name": "",
		"sent":      "2",
		"received":  "0",
		"rta":       "",
		"pl":        "100",
		"ttl":       "",
	}
	// debian 12
	out := `
PING 10.0.1.11 (10.0.1.11) 56(84) bytes of data.
From 10.0.2.1 icmp_seq=1 Destination Host Unreachable
From 10.0.2.1 icmp_seq=2 Destination Host Unreachable

--- 10.0.1.11 ping statistics ---
2 packets transmitted, 0 received, +2 errors, 100% packet loss, time 1002ms
`
	chk := &CheckPing{}
	entry := chk.parsePingOutput(out, "")
	delete(entry, "_error")
	assert.Equalf(t, exp, entry, "parsed ping ok output")
}

func TestPingParserOSXOK(t *testing.T) {
	exp := map[string]string{
		"host_name": "",
		"sent":      "5",
		"received":  "5",
		"rta":       "0.066",
		"pl":        "0.0",
		"ttl":       "64",
	}
	// osx 14.7
	out := `
PING localhost (127.0.0.1): 56 data bytes
64 bytes from 127.0.0.1: icmp_seq=0 ttl=64 time=0.040 ms
64 bytes from 127.0.0.1: icmp_seq=1 ttl=64 time=0.095 ms
64 bytes from 127.0.0.1: icmp_seq=2 ttl=64 time=0.086 ms
64 bytes from 127.0.0.1: icmp_seq=3 ttl=64 time=0.060 ms
64 bytes from 127.0.0.1: icmp_seq=4 ttl=64 time=0.051 ms

--- localhost ping statistics ---
5 packets transmitted, 5 packets received, 0.0% packet loss
round-trip min/avg/max/stddev = 0.040/0.066/0.095/0.021 ms
`
	chk := &CheckPing{}
	entry := chk.parsePingOutput(out, "")
	delete(entry, "_error")
	assert.Equalf(t, exp, entry, "parsed ping ok output")
}

func TestPingParserOSXBad(t *testing.T) {
	exp := map[string]string{
		"host_name": "",
		"sent":      "5",
		"received":  "0",
		"rta":       "",
		"pl":        "100.0",
		"ttl":       "",
	}
	// osx 14.7
	out := `
PING 10.99.99.99 (10.99.99.99): 56 data bytes
Request timeout for icmp_seq 0
Request timeout for icmp_seq 1
Request timeout for icmp_seq 2
Request timeout for icmp_seq 3

--- 10.99.99.99 ping statistics ---
5 packets transmitted, 0 packets received, 100.0% packet loss
`
	chk := &CheckPing{}
	entry := chk.parsePingOutput(out, "")
	delete(entry, "_error")
	assert.Equalf(t, exp, entry, "parsed ping ok output")
}
