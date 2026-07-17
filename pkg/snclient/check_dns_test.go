//go:build !windows

package snclient

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startTestDNSServerHandler starts a local DNS server on the given address
// using the given handler.
// It returns the port it has started on as string
func startTestDNSServerHandler(t *testing.T, listenAddr string, handler dns.Handler) string {
	t.Helper()

	pc, err := net.ListenPacket("udp", listenAddr)
	require.NoError(t, err)

	srv := &dns.Server{
		PacketConn: pc,
		Handler:    handler,
	}
	go func() {
		_ = srv.ActivateAndServe()
	}()
	t.Cleanup(func() {
		_ = srv.Shutdown()
	})

	udpAddr, ok := pc.LocalAddr().(*net.UDPAddr)
	require.True(t, ok, "local addr is a udp addr")

	return strconv.Itoa(udpAddr.Port)
}

// startTestDNSServer starts a local DNS server on the given address
// it answers every query with the given rcode and no answer records.
// It returns the port the port it has started on as string
func startTestDNSServer(t *testing.T, listenAddr string, rcode int) string {
	t.Helper()

	return startTestDNSServerHandler(t, listenAddr, dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		reply := new(dns.Msg)
		reply.SetRcode(req, rcode)
		_ = w.WriteMsg(reply)
	}))
}

// startSilentDNSServer opens a udp listener on the given address which never
// responds, causing DNS query timeouts.
// It returns the port it has started on as string
func startSilentDNSServer(t *testing.T, listenAddr string) string {
	t.Helper()

	pc, err := net.ListenPacket("udp", listenAddr)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = pc.Close()
	})

	udpAddr, ok := pc.LocalAddr().(*net.UDPAddr)
	require.True(t, ok, "local addr is a udp addr")

	return strconv.Itoa(udpAddr.Port)
}

func TestCheckDNS(t *testing.T) {
	config := `
[/modules]
CheckBuiltinPlugins = enabled
	`
	snc := StartTestAgent(t, config)

	t.Run("basic a lookup", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de"})
		assert.Equalf(t, CheckExitOK, res.State, "state ok")
		assert.Regexpf(
			t,
			`^OK - labs\.consol\.de returns 94\.185\.89\.33`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("expected string all match", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-e", "94.185.89.33"})
		assert.Equalf(t, CheckExitOK, res.State, "state ok")
		assert.Regexpf(
			t,
			`^OK - labs\.consol\.de returns 94\.185\.89\.33`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("expected string extra expected", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-e", "94.185.89.33", "-e", "1.2.3.4"})
		assert.Equalf(t, CheckExitWarning, res.State, "state warning")
		assert.Regexpf(
			t,
			`^WARNING - labs\.consol\.de returns 94\.185\.89\.33`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("expected string missing expected", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-e", "94.185.89.33", "-e", "1.2.3.4", "-e", "5.6.7.8"})
		assert.Equalf(t, CheckExitWarning, res.State, "state warning")
		assert.Regexpf(
			t,
			`^WARNING - labs\.consol\.de returns 94\.185\.89\.33`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("expected string none match", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-e", "1.2.3.4"})
		assert.Equalf(t, CheckExitCritical, res.State, "state critical")
		assert.Regexpf(
			t,
			`^CRITICAL - labs\.consol\.de returns 94\.185\.89\.33`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("warning threshold not triggered", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-w", "999"})
		assert.Equalf(t, CheckExitOK, res.State, "state ok")
		assert.Regexpf(
			t,
			`^OK`,
			string(res.BuildPluginOutput()),
			"not warned",
		)
	})

	t.Run("critical threshold triggered", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-c", "0"})
		assert.Equalf(t, CheckExitCritical, res.State, "state critical")
		assert.Regexpf(
			t,
			`^CRITICAL`,
			string(res.BuildPluginOutput()),
			"critical threshold triggered",
		)
	})

	t.Run("aaaa lookup", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-q", "AAAA"})
		assert.Equalf(t, CheckExitOK, res.State, "state ok")
		assert.Regexpf(
			t,
			`^OK - labs\.consol\.de returns 2a03:3680:0:2::21 \(AAAA\)`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("aaaa expected string match", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-q", "AAAA", "-e", "2a03:3680:0:2::21"})
		assert.Equalf(t, CheckExitOK, res.State, "state ok")
		assert.Regexpf(
			t,
			`^OK - labs\.consol\.de returns 2a03:3680:0:2::21`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("multiple answers all match", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "one.one.one.one", "-e", "1.1.1.1", "-e", "1.0.0.1"})
		assert.Equalf(t, CheckExitOK, res.State, "state ok")
		assert.Regexpf(
			t,
			`^OK - one\.one\.one\.one returns (1\.1\.1\.1|1\.0\.0\.1)`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	// Resolves cloudflares one.one.one.one using whatever nameserver is configured, not the 1.1.1.1 DNS namesever

	t.Run("multiple answers partial match", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "one.one.one.one", "-e", "1.1.1.1"})
		assert.Equalf(t, CheckExitWarning, res.State, "state warning")
		assert.Regexpf(
			t,
			`^WARNING - one\.one\.one\.one returns (1\.1\.1\.1|1\.0\.0\.1)`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	// Resolves cloudflares one.one.one.one using whatever nameserver is configured, not the 1.1.1.1 DNS namesever

	t.Run("multiple answers none match", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "one.one.one.one", "-e", "8.8.8.8"})
		assert.Equalf(t, CheckExitCritical, res.State, "state critical")
		assert.Regexpf(
			t,
			`^CRITICAL - one\.one\.one\.one returns (1\.1\.1\.1|1\.0\.0\.1)`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("nxdomain rcode", func(t *testing.T) {
		port := startTestDNSServer(t, "127.0.0.1:0", dns.RcodeNameError)
		res := snc.RunCheck("check_dns", []string{"-H", "nxdomain.example.com.", "-s", "127.0.0.1", "-p", port})
		assert.Equalf(t, CheckExitCritical, res.State, "state critical")
		assert.Regexpf(
			t,
			`^CRITICAL - dns lookup failed for host 'nxdomain\.example\.com':\n127\.0\.0\.1:`+port+`: NXDOMAIN$`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("servfail rcode", func(t *testing.T) {
		port := startTestDNSServer(t, "127.0.0.1:0", dns.RcodeServerFailure)
		res := snc.RunCheck("check_dns", []string{"-H", "servfail.example.com.", "-s", "127.0.0.1", "-p", port})
		assert.Equalf(t, CheckExitCritical, res.State, "state critical")
		assert.Regexpf(
			t,
			`^CRITICAL - dns lookup failed for host 'servfail\.example\.com':\n127\.0\.0\.1:`+port+`: SERVFAIL$`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("empty noerror answer", func(t *testing.T) {
		port := startTestDNSServer(t, "127.0.0.1:0", dns.RcodeSuccess)
		res := snc.RunCheck("check_dns", []string{"-H", "nodata.example.com.", "-s", "127.0.0.1", "-p", port})
		assert.Equalf(t, CheckExitCritical, res.State, "state critical")
		assert.Regexpf(
			t,
			`^CRITICAL - dns lookup failed for host 'nodata\.example\.com':\n127\.0\.0\.1:`+port+`: no answer \(NOERROR\)$`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("empty results unique across search paths", func(t *testing.T) {
		port := startTestDNSServer(t, "127.0.0.1:0", dns.RcodeNameError)
		res := snc.RunCheck("check_dns", []string{
			"-H", "myhost",
			"--search-path", "one.example.com", "--search-path", "two.example.com",
			"-s", "127.0.0.1", "-p", port,
		})
		assert.Equalf(t, CheckExitCritical, res.State, "state critical")
		assert.Regexpf(
			t,
			`^CRITICAL - dns lookup failed for host 'myhost':\n127\.0\.0\.1:`+port+`: NXDOMAIN$`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("empty results from multiple nameservers", func(t *testing.T) {
		port := startTestDNSServer(t, "127.0.0.1:0", dns.RcodeServerFailure)
		pc, err := net.ListenPacket("udp", "127.0.0.2:"+port)
		if err != nil {
			t.Skipf("cannot listen on 127.0.0.2: %s", err)
		}
		_ = pc.Close()
		startTestDNSServer(t, "127.0.0.2:"+port, dns.RcodeNameError)

		resolvConf := filepath.Join(t.TempDir(), "resolv.conf")
		require.NoError(t, os.WriteFile(resolvConf, []byte("nameserver 127.0.0.1\nnameserver 127.0.0.2\n"), 0o600))

		res := snc.RunCheck("check_dns", []string{
			"-H", "multi.example.com.",
			"--resolv-conf-file", resolvConf,
			"-p", port,
		})
		assert.Equalf(t, CheckExitCritical, res.State, "state critical")
		output := string(res.BuildPluginOutput())
		assert.Regexpf(
			t,
			`^CRITICAL - dns lookup failed for host 'multi\.example\.com':\n127\.0\.0\.1:`+port+`: SERVFAIL`,
			output,
			"first nameserver gets its own line",
		)
		assert.Containsf(t, output, "127.0.0.2:"+port+": NXDOMAIN", "second nameserver gets its own line")
	})

	t.Run("query timeout on all nameservers", func(t *testing.T) {
		port := startSilentDNSServer(t, "127.0.0.1:0")
		res := snc.RunCheck("check_dns", []string{"-H", "silent.example.com.", "-s", "127.0.0.1", "-p", port, "-T", "1"})
		assert.Equalf(t, CheckExitCritical, res.State, "state critical")
		assert.Regexpf(
			t,
			`^CRITICAL - dns lookup failed for host 'silent\.example\.com':\n127\.0\.0\.1:`+port+`: query failed: timeout$`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("query timeout continues with next nameserver", func(t *testing.T) {
		port := startSilentDNSServer(t, "127.0.0.1:0")
		pc, err := net.ListenPacket("udp", "127.0.0.2:"+port)
		if err != nil {
			t.Skipf("cannot listen on 127.0.0.2: %s", err)
		}
		_ = pc.Close()
		startTestDNSServerHandler(t, "127.0.0.2:"+port, dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
			reply := new(dns.Msg)
			reply.SetReply(req)
			rr, rrErr := dns.NewRR(req.Question[0].Name + " 60 IN A 1.2.3.4")
			if rrErr == nil {
				reply.Answer = append(reply.Answer, rr)
			}
			_ = w.WriteMsg(reply)
		}))

		res := snc.RunCheck("check_dns", []string{
			"-H", "slow.example.com.",
			"-s", "127.0.0.1", "-s", "127.0.0.2",
			"-p", port,
			"-T", "1",
		})
		assert.Equalf(t, CheckExitOK, res.State, "state ok")
		assert.Regexpf(
			t,
			`^OK - slow\.example\.com\. returns 1\.2\.3\.4 \(A\)`,
			string(res.BuildPluginOutput()),
			"output matches",
		)
	})

	t.Run("missing host argument", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - .*host.*`,
			string(res.BuildPluginOutput()),
			"missing host argument",
		)
	})

	t.Run("empty host argument", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", ""})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - host must not be empty`,
			string(res.BuildPluginOutput()),
			"empty host argument",
		)
	})

	t.Run("empty query type", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-q", " "})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - query type must not be empty`,
			string(res.BuildPluginOutput()),
			"empty query type",
		)
	})

	t.Run("invalid port", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-p", "70000"})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - port must be between 1 and 65535, got: 70000`,
			string(res.BuildPluginOutput()),
			"invalid port",
		)
	})

	t.Run("zero timeout", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-t", "0"})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - timeout must be a positive number of seconds, got: 0`,
			string(res.BuildPluginOutput()),
			"zero timeout",
		)
	})

	t.Run("negative timeout", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-t", "-5"})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - timeout must be a positive number of seconds, got: -5`,
			string(res.BuildPluginOutput()),
			"negative timeout",
		)
	})

	t.Run("zero query timeout", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-T", "0"})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - query timeout must be a positive number of seconds, got: 0`,
			string(res.BuildPluginOutput()),
			"zero query timeout",
		)
	})

	t.Run("negative warning threshold", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-w", "-1"})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - warning threshold must not be negative, got: -1`,
			string(res.BuildPluginOutput()),
			"negative warning threshold",
		)
	})

	t.Run("negative critical threshold", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-c", "-1"})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - critical threshold must not be negative, got: -1`,
			string(res.BuildPluginOutput()),
			"negative critical threshold",
		)
	})

	t.Run("warning threshold higher than critical", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-w", "10", "-c", "5"})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - warning threshold \(10\) must not be higher than the critical threshold \(5\)`,
			string(res.BuildPluginOutput()),
			"warning threshold higher than critical",
		)
	})

	t.Run("empty expected string", func(t *testing.T) {
		res := snc.RunCheck("check_dns", []string{"-H", "labs.consol.de", "-e", ""})
		assert.Equalf(t, CheckExitUnknown, res.State, "state unknown")
		assert.Regexpf(
			t,
			`^UNKNOWN - expected string must not be empty`,
			string(res.BuildPluginOutput()),
			"empty expected string",
		)
	})

	StopTestAgent(t, snc)
}
