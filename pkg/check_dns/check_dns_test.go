package check_dns

import (
	"errors"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckDNSAnswer(t *testing.T) {
	tests := []struct {
		record string
		answer string
		kind   string
	}{
		{record: "example.com. 60 IN A 192.0.2.1", answer: "192.0.2.1", kind: "A"},
		{record: "example.com. 60 IN AAAA 2001:db8::1", answer: "2001:db8::1", kind: "AAAA"},
		{record: "example.com. 60 IN MX 10 mail.example.com.", answer: "mail.example.com.", kind: "MX"},
		{record: "example.com. 60 IN CNAME alias.example.com.", answer: "alias.example.com.", kind: "CNAME"},
		{record: "example.com. 60 IN TXT \"hello\""},
	}

	for _, test := range tests {
		t.Run(test.record, func(t *testing.T) {
			record, err := dns.NewRR(test.record)
			require.NoError(t, err)
			answer, kind, err := dnsAnswer(record)
			if test.kind == "" {
				require.ErrorContains(t, err, "is not a supported query type")
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, test.answer, answer)
			assert.Equal(t, test.kind, kind)
		})
	}
}

func TestCheckDNSEmptyResultReason(t *testing.T) {
	assert.Equal(t, "no answer (NOERROR)", emptyResultReason(dns.RcodeSuccess))
	assert.Equal(t, "NXDOMAIN", emptyResultReason(dns.RcodeNameError))
	assert.Equal(t, "SERVFAIL", emptyResultReason(dns.RcodeServerFailure))
	assert.Equal(t, "RCODE 999", emptyResultReason(999))
}

func TestCheckDNSQueryFailedReason(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		reason string
	}{
		{name: "plain error", err: errors.New("connection refused"), reason: "query failed: connection refused"},
		{name: "nested error", err: fmt.Errorf("query: %w", &net.OpError{Op: "read", Err: errors.New("connection refused")}), reason: "query failed: connection refused"},
		{name: "wrapped timeout", err: fmt.Errorf("query: %w", os.ErrDeadlineExceeded), reason: "query failed: timeout"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.reason, queryFailedReason(test.err))
		})
	}
}

func TestCheckDNSEmptyResultsMessage(t *testing.T) {
	tests := []struct {
		name        string
		nameservers []string
		results     []emptyResult
		message     string
	}{
		{
			name:        "no results",
			nameservers: []string{"192.0.2.1:53"},
			message:     "all DNS queries gave empty results or failed",
		},
		{
			name:        "duplicate reasons and unused server",
			nameservers: []string{"192.0.2.1:53", "192.0.2.2:53"},
			results: []emptyResult{
				{host: "example.com.", nameserver: "192.0.2.1:53", reason: "NXDOMAIN"},
				{host: "example.com.search.", nameserver: "192.0.2.1:53", reason: "NXDOMAIN"},
				{host: "example.com.other.", nameserver: "192.0.2.1:53", reason: "SERVFAIL"},
			},
			message: "DNS lookup failed for host 'example.com': 192.0.2.1:53: NXDOMAIN, SERVFAIL",
		},
		{
			name:        "sorted servers",
			nameservers: []string{"192.0.2.2:53", "192.0.2.1:53"},
			results: []emptyResult{
				{host: "example.com.", nameserver: "192.0.2.2:53", reason: "query failed: timeout"},
				{host: "example.com.", nameserver: "192.0.2.1:53", reason: "NXDOMAIN"},
			},
			message: "DNS lookup failed for host 'example.com':\n192.0.2.1:53: NXDOMAIN\n192.0.2.2:53: query failed: timeout",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.message, emptyResultsMessage("example.com.", test.nameservers, test.results))
		})
	}
}
