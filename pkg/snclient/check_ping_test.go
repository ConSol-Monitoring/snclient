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
	assert.Regexpf(t, `^OK - Packet loss = [\d.]+%, RTA = [\d.]+ms \|'rta'=[\d.]+ms;1000;5000 'pl'=\d+%;30;80;0`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_ping", []string{"host=10.99.99.99"})
	assert.Equalf(t, CheckExitCritical, res.State, "state critical")
	assert.Regexpf(t, `^CRITICAL - Packet loss = 100(|\.0)% \|'rta'=U 'pl'=100%;30;80;0`, string(res.BuildPluginOutput()), "output matches")

	res = snc.RunCheck("check_ping", []string{"host=should_not_resolve.nowhere"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
	assert.Regexpf(t, `^UNKNOWN - failed to resolve hostname`, string(res.BuildPluginOutput()), "output matches")

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

func TestPingParserLinuxAlpineOK(t *testing.T) {
	exp := map[string]string{
		"host_name": "",
		"sent":      "2",
		"received":  "2",
		"rta":       "0.083",
		"pl":        "0",
		"ttl":       "64",
	}
	// alpine 3.19
	out := `
PING localhost (::1): 56 data bytes
64 bytes from ::1: seq=0 ttl=64 time=0.067 ms
64 bytes from ::1: seq=1 ttl=64 time=0.100 ms

--- localhost ping statistics ---
2 packets transmitted, 2 packets received, 0% packet loss
round-trip min/avg/max = 0.067/0.083/0.100 ms
`
	chk := &CheckPing{}
	entry := chk.parsePingOutput(out, "")
	assert.Equalf(t, exp, entry, "parsed ping ok output")
}

func TestPingParserLinuxAlpineBad(t *testing.T) {
	exp := map[string]string{
		"host_name": "",
		"sent":      "2",
		"received":  "0",
		"rta":       "",
		"pl":        "100",
		"ttl":       "",
	}
	// alpine 3.19
	out := `
PING 192.168.123.123 (192.168.123.123): 56 data bytes

--- 192.168.123.123 ping statistics ---
2 packets transmitted, 0 packets received, 100% packet loss
`
	chk := &CheckPing{}
	entry := chk.parsePingOutput(out, "")
	delete(entry, "_error")
	assert.Equalf(t, exp, entry, "parsed ping ok output")
}

func TestPingParserLinuxAlpineBad2(t *testing.T) {
	exp := map[string]string{
		"host_name": "",
		"sent":      "",
		"received":  "",
		"rta":       "",
		"pl":        "100",
		"ttl":       "",
	}
	// alpine 3.19
	out := `
ping: bad address 'does.not.exist'
`
	chk := &CheckPing{}
	entry := chk.parsePingOutput(out, "")
	delete(entry, "_error")
	assert.Equalf(t, exp, entry, "parsed ping ok output")
}

func TestPingParserWindowsOK(t *testing.T) {
	exp := map[string]string{
		"host_name": "",
		"sent":      "3",
		"received":  "3",
		"rta":       "7",
		"pl":        "0",
		"ttl":       "127",
	}
	// windows 10
	out := `
Pinging 10.0.2.1 with 32 bytes of data:
Reply from 10.0.2.1: bytes=32 time=11ms TTL=127
Reply from 10.0.2.1: bytes=32 time=5ms TTL=127
Reply from 10.0.2.1: bytes=32 time=5ms TTL=127

Ping statistics for 10.0.2.1:
    Packets: Sent = 3, Received = 3, Lost = 0 (0% loss),
Approximate round trip times in milli-seconds:
    Minimum = 5ms, Maximum = 11ms, Average = 7ms
`
	chk := &CheckPing{}
	entry := chk.parsePingOutput(out, "")
	delete(entry, "_error")
	assert.Equalf(t, exp, entry, "parsed ping ok output")
}

func TestPingParserWindowsBad(t *testing.T) {
	exp := map[string]string{
		"host_name": "",
		"sent":      "4",
		"received":  "4",
		"rta":       "",
		"pl":        "100",
		"ttl":       "",
	}
	// windows 10
	out := `
Pinging 10.0.0.1 with 32 bytes of data:
Reply from 82.135.16.21: Destination net unreachable.
Reply from 82.135.16.21: Destination net unreachable.
Reply from 82.135.16.21: Destination net unreachable.
Reply from 82.135.16.21: Destination net unreachable.

Ping statistics for 10.0.0.1:
Packets: Sent = 4, Received = 4, Lost = 0 (0% loss),
`
	chk := &CheckPing{}
	entry := chk.parsePingOutput(out, "")
	delete(entry, "_error")
	assert.Equalf(t, exp, entry, "parsed ping ok output")
}

func TestPingParserWindowsBad2(t *testing.T) {
	exp := map[string]string{
		"host_name": "",
		"sent":      "3",
		"received":  "0",
		"rta":       "",
		"pl":        "100",
		"ttl":       "",
	}
	// windows 10
	out := `
Pinging 4.4.4.4 with 32 bytes of data:
Request timed out.
Request timed out.
Request timed out.

Ping statistics for 4.4.4.4:
Packets: Sent = 3, Received = 0, Lost = 3 (100% loss),
	`
	chk := &CheckPing{}
	entry := chk.parsePingOutput(out, "")
	delete(entry, "_error")
	assert.Equalf(t, exp, entry, "parsed ping ok output")
}

func TestPingParserWindowsOKDE(t *testing.T) {
	exp := map[string]string{
		"host_name": "",
		"sent":      "5",
		"received":  "5",
		"rta":       "6",
		"pl":        "0",
		"ttl":       "",
	}
	// windows 10
	out := `
Ping wird ausgeführt für test-12345 [::1] mit 32 Bytes Daten:
Antwort von ::1: Zeit<1ms
Antwort von ::1: Zeit<1ms
Antwort von ::1: Zeit<1ms
Antwort von ::1: Zeit<1ms
Antwort von ::1: Zeit<1ms

Ping-Statistik für ::1:
    Pakete: Gesendet = 5, Empfangen = 5, Verloren = 0
    (0% Verlust),
Ca. Zeitangaben in Millisek.:
    Minimum = 3ms, Maximum = 9ms, Mittelwert = 6ms
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
