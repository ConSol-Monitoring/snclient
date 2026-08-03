package check_http

import (
	"container/heap"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Issues that are more important to display have lower importance numbers.
const (
	resultImportancePerLevel         = 100
	notAfterImportanceLevel          = 51
	notBeforeImportanceLevel         = 52
	signatureImportanceLevel         = 53
	subjectAlternativeNameImportance = 54
	commonNameImportance             = 55
)

// checkCertificate establishes a TLS connection to the server and validates the certificate against the warning and critical thresholds.
// It returns immediately without checking the HTTP content.
func checkCertificate(ctx context.Context, opts *commandOpts, dialFunc func(ctx context.Context, _ string, _ string) (net.Conn, error), tlsConfig *tls.Config) *CheckResult {
	// For certificate checking, we need to set ServerName for SNI
	if tlsConfig.ServerName == "" {
		host, _, err := net.SplitHostPort(opts.flags.Hostname)
		if err != nil {
			host = opts.flags.Hostname
		}

		tlsConfig.ServerName = host
	}

	conn, err := dialFunc(ctx, "", "")
	if err != nil {
		return &CheckResult{
			nil,
			fmt.Sprintf("HTTP CRITICAL - Error connecting to host %s on port %d: %v", opts.flags.IPAddress, opts.flags.Port, err),
			CRITICAL,
		}
	}
	defer conn.Close()

	tlsConn := tls.Client(conn, tlsConfig)

	handshakeErr := tlsConn.HandshakeContext(ctx)
	if handshakeErr != nil {
		return &CheckResult{
			nil,
			fmt.Sprintf("HTTP CRITICAL - TLS handshake failed for host %s on port %d: %v", opts.flags.IPAddress, opts.flags.Port, handshakeErr),
			CRITICAL,
		}
	}

	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return &CheckResult{
			nil,
			fmt.Sprintf("HTTP CRITICAL - No certificate returned from host %s on port %d", opts.flags.IPAddress, opts.flags.Port),
			CRITICAL,
		}
	}

	// certs[0] is the leaf certificate, the certificate belonging to the site that we are visiting
	// certs[1..n-1] are the intermediate certificates sign each other and go up in scope
	// certs[n] is the root certificate. this is either from the web browser / system
	// use a dedicated function to check the chain, the logic is too long

	return checkCertificateChain(opts, certs)
}

// The main inspiration is from https://github.com/matteocorti/check_ssl_cert.
// That project has many options, this function implements only a subset of them.
//
//nolint:gocognit,funlen // the function logic is simple
func checkCertificateChain(opts *commandOpts, certs []*x509.Certificate) *CheckResult {
	// OK - Certificate 'se1-mon-q001.sys.schwarz' will expire on Sat 27 May 2028 04:55:09 PM GMT +0000 (expires in X days)
	const customTimeLayout = "Mon 02 Jan 2006 03:04:05 PM MST -0700"

	resultsPQ := &CheckResultPQ{}
	heap.Init(resultsPQ)

	var perfParts []string

	critDaysPerfStr := ""
	if opts.certificateCritDays != nil {
		critDaysPerfStr = strconv.Itoa(*opts.certificateCritDays)
	}

	// Determine the hostname to match against the certificate's CN and SANs.
	// When SNI is enabled the TLS ServerName is already set in the tls.Config,
	// but we derive it from opts.Hostname here to match consistently.
	matchHostname := opts.flags.Hostname

	host, _, splitErr := net.SplitHostPort(opts.flags.Hostname)
	if opts.flags.SNI && splitErr == nil {
		matchHostname = host
	}

	for idx, cert := range certs {
		shouldCheck := idx == 0 || !opts.flags.IgnoreCertificateChain
		// the output of the check_ssl_cert tool indexes from 1
		perfIndex := idx + 1

		// Expiry check and perfdata.
		expiry := cert.NotAfter
		daysLeft := int(time.Until(expiry).Hours() / hoursInDays)

		//nolint:nestif // imported 3rd party source
		if shouldCheck {
			perfParts = append(perfParts, fmt.Sprintf("days_chain_elem%d=%d;%d;%s;0", perfIndex, daysLeft, opts.certificateWarnDays, critDaysPerfStr))

			if opts.flags.CheckCN {
				pushCommonNameCheck(cert, matchHostname, perfIndex, resultsPQ, opts)
			}

			if opts.flags.CheckSAN {
				pushSubjectAlternativeNameCheck(cert, matchHostname, perfIndex, resultsPQ, opts)
			}

			if !opts.flags.IgnoreNotBefore {
				pushNotBeforeCheck(cert, perfIndex, customTimeLayout, resultsPQ, opts)
			}

			if !opts.flags.IgnoreNotAfter {
				pushNotAfterCheck(cert, opts, perfIndex, customTimeLayout, resultsPQ)
			}

			if !opts.flags.IgnoreSignatureAlgorithm {
				// Signature algorithm check.
				pushSignatureCheck(cert, perfIndex, resultsPQ, opts)
			}
		} else {
			perfParts = append(perfParts, fmt.Sprintf("days_chain_elem%d=%d;;;0", perfIndex, daysLeft))
		}
	}

	subchecks := []*CheckResult{}

	for resultsPQ.Len() > 0 {
		top, ok := heap.Pop(resultsPQ).(*CheckResult)
		if !ok {
			break
		}

		subchecks = append(subchecks, top)
	}

	if opts.flags.Verbose {
		for sIdx, subcheck := range subchecks {
			importanceStr := "undefined"
			if subcheck.resultImportance != nil {
				importanceStr = strconv.FormatInt(int64(*subcheck.resultImportance), 10)
			}

			opts.tracef("subcheck %d | code: %d | importance: %s | msg: %s", sIdx, subcheck.code, importanceStr, subcheck.msg)
		}
	}

	if len(subchecks) == 0 {
		return &CheckResult{
			nil,
			"HTTP UNKNOWN - Internal error during certificate check: unexpected type in priority queue",
			UNKNOWN,
		}
	}

	top := subchecks[0]

	perfData := strings.Join(perfParts, " ")
	if perfData != "" {
		top.msg += " | " + perfData
	}

	return top
}

// formatCertSubject returns a formatted string with the certificate subject details.
func formatCertSubject(cert *x509.Certificate) string {
	return fmt.Sprintf("'%s' from '%s'", cert.Subject.CommonName, cert.Issuer.CommonName)
}

// Taken from:  /usr/local/go/src/crypto/x509/verify.go as it was not exported.
// Useful for checking CommonName
// validHostname reports whether host is a valid hostname that can be matched or
// matched against according to RFC 6125 2.2, with some leniency to accommodate
// legacy values.
//
//nolint:all // taken from the standard library, not going to change the function
func validHostname(host string, isPattern bool) bool {
	if !isPattern {
		host = strings.TrimSuffix(host, ".")
	}
	if len(host) == 0 {
		return false
	}
	if host == "*" {
		// Bare wildcards are not allowed, they are not valid DNS names,
		// nor are they allowed per RFC 6125.
		return false
	}

	for i, part := range strings.Split(host, ".") {
		if part == "" {
			// Empty label.
			return false
		}
		if isPattern && i == 0 && part == "*" {
			// Only allow full left-most wildcards, as those are the only ones
			// we match, and matching literal '*' characters is probably never
			// the expected behavior.
			continue
		}
		for j, c := range part {
			if 'a' <= c && c <= 'z' {
				continue
			}
			if '0' <= c && c <= '9' {
				continue
			}
			if 'A' <= c && c <= 'Z' {
				continue
			}
			if c == '-' && j != 0 {
				continue
			}
			if c == '_' {
				// Not a valid character in hostnames, but commonly
				// found in deployments outside the WebPKI.
				continue
			}
			return false
		}
	}

	return true
}

// taken from: /usr/local/go/src/crypto/x509/verify.go as it was not exported
// Useful for checking CommonName
//
//nolint:all // taken from the standard library, not going to change the function
func matchHostnames(pattern, host string) bool {
	pattern = toLowerCaseASCII(pattern)
	host = toLowerCaseASCII(strings.TrimSuffix(host, "."))

	if len(pattern) == 0 || len(host) == 0 {
		return false
	}

	patternParts := strings.Split(pattern, ".")
	hostParts := strings.Split(host, ".")

	if len(patternParts) != len(hostParts) {
		return false
	}

	for i, patternPart := range patternParts {
		if i == 0 && patternPart == "*" {
			continue
		}
		if patternPart != hostParts[i] {
			return false
		}
	}

	return true
}

// taken from: /usr/local/go/src/crypto/x509/verify.go as it was not exported
// toLowerCaseASCII returns a lower-case version of in. See RFC 6125 6.4.1. We use
// an explicitly ASCII function to avoid any sharp corners resulting from
// performing Unicode operations on DNS labels.
//
//nolint:all // taken from the standard library, not going to change the function
func toLowerCaseASCII(in string) string {
	// If the string is already lower-case then there's nothing to do.
	isAlreadyLowerCase := true
	for _, c := range in {
		if c == utf8.RuneError {
			// If we get a UTF-8 error then there might be
			// upper-case ASCII bytes in the invalid sequence.
			isAlreadyLowerCase = false
			break
		}
		if 'A' <= c && c <= 'Z' {
			isAlreadyLowerCase = false
			break
		}
	}

	if isAlreadyLowerCase {
		return in
	}

	out := []byte(in)
	for i, c := range out {
		if 'A' <= c && c <= 'Z' {
			out[i] += 'a' - 'A'
		}
	}
	return string(out)
}

func pushCommonNameCheck(cert *x509.Certificate, hostname string, index int, resultsPQ *CheckResultPQ, opts *commandOpts) {
	resultImportance := index*resultImportancePerLevel + commonNameImportance

	if cert.IsCA {
		heap.Push(resultsPQ, &CheckResult{
			&resultImportance,
			fmt.Sprintf("HTTP OK - x509 certificate %s is a CA certificate, skipping common name check for hostname %q",
				formatCertSubject(cert), hostname),
			OK,
		})

		return
	}

	cnIsValid := validHostname(cert.Subject.CommonName, false) || validHostname(hostname, true)
	if !cnIsValid {
		heap.Push(resultsPQ, &CheckResult{
			&resultImportance,
			fmt.Sprintf("HTTP CRITICAL - x509 certificate %s has common name %q, which is not a valid pattern for a common name",
				formatCertSubject(cert), cert.Subject.CommonName),
			CRITICAL,
		})

		opts.tracef("certificate check: CN invalid pattern for cert %s", formatCertSubject(cert))
	}

	cnMatchesHostname := matchHostnames(cert.Subject.CommonName, hostname)
	if !cnMatchesHostname {
		heap.Push(resultsPQ, &CheckResult{
			&resultImportance,
			fmt.Sprintf("HTTP CRITICAL - x509 certificate %s has common name %q, which does not match hostname %q",
				formatCertSubject(cert), cert.Subject.CommonName, hostname),
			CRITICAL,
		})

		opts.tracef("certificate check: CN %q does not match hostname %q for cert %s", cert.Subject.CommonName, hostname, formatCertSubject(cert))
	}

	heap.Push(resultsPQ, &CheckResult{
		&resultImportance,
		fmt.Sprintf("HTTP OK - x509 certificate %s has common name %q, which matches hostname %q",
			formatCertSubject(cert), cert.Subject.CommonName, hostname),
		OK,
	})
}

// pushSubjectAlternativeNameCheck verifies that the certificate's IP or DNS SAN names
// match the expected hostname (or SNI name).
func pushSubjectAlternativeNameCheck(cert *x509.Certificate, hostname string, index int, resultsPQ *CheckResultPQ, opts *commandOpts) {
	resultImportance := index*resultImportancePerLevel + subjectAlternativeNameImportance

	if cert.IsCA {
		heap.Push(resultsPQ, &CheckResult{
			&resultImportance,
			fmt.Sprintf("HTTP OK - x509 certificate %s is a CA certificate, skipping SAN check for hostname %q - (IP SANs: %v, DNS SANs: %v)",
				formatCertSubject(cert), hostname, cert.IPAddresses, cert.DNSNames),
			OK,
		})

		return
	}

	// verifyHostname ignores legacy CommonName field
	// it checks using x509.Certificate.IPAddresses (IP SANs)
	// or  x509.Certificate.DNSNames (Hostname SANs)
	err := cert.VerifyHostname(hostname)
	if err != nil {
		heap.Push(resultsPQ, &CheckResult{
			&resultImportance,
			fmt.Sprintf("HTTP CRITICAL - x509 certificate %s has IP/DNS SANs that do not match hostname %q - (IP SANs: %v, DNS SANs: %v)",
				formatCertSubject(cert), hostname, cert.IPAddresses, cert.DNSNames),
			CRITICAL,
		})

		opts.tracef("certificate check: SANs do not match hostname %q for cert %s", hostname, formatCertSubject(cert))

		return
	}

	heap.Push(resultsPQ, &CheckResult{
		&resultImportance,
		fmt.Sprintf("HTTP OK - x509 certificate %s has IP/DNS SANs that match hostname %q - (IP SANs: %v, DNS SANs: %v)",
			formatCertSubject(cert), hostname, cert.IPAddresses, cert.DNSNames),
		OK,
	})
}

// pushNotAfterCheck checks the certificate's NotAfter expiry against warning/critical thresholds.
func pushNotAfterCheck(cert *x509.Certificate, opts *commandOpts, index int, timeLayout string, resultsPQ *CheckResultPQ) {
	resultImportance := index*resultImportancePerLevel + notAfterImportanceLevel

	expiry := cert.NotAfter
	daysLeft := int(time.Until(expiry).Hours() / hoursInDays)

	switch {
	case opts.certificateCritDays != nil && daysLeft <= *opts.certificateCritDays:
		heap.Push(resultsPQ, &CheckResult{
			&resultImportance,
			fmt.Sprintf("HTTP CRITICAL - x509 certificate %s is valid until %s (expires in %d days)",
				formatCertSubject(cert), expiry.Format(timeLayout), daysLeft),
			CRITICAL,
		})

		opts.tracef("certificate check: cert %s expires in %d days (critical)", formatCertSubject(cert), daysLeft)
	case daysLeft <= opts.certificateWarnDays:
		heap.Push(resultsPQ, &CheckResult{
			&resultImportance,
			fmt.Sprintf("HTTP WARNING - x509 certificate %s is valid until %s (expires in %d days)",
				formatCertSubject(cert), expiry.Format(timeLayout), daysLeft),
			WARNING,
		})

		opts.tracef("certificate check: cert %s expires in %d days (warning)", formatCertSubject(cert), daysLeft)
	default:
		heap.Push(resultsPQ, &CheckResult{
			&resultImportance,
			fmt.Sprintf("HTTP OK - x509 certificate %s is valid until %s (expires in %d days)",
				formatCertSubject(cert), expiry.Format(timeLayout), daysLeft),
			OK,
		})
	}
}

// pushSignatureCheck validates that the certificate is not signed using a weak algorithm.
func pushSignatureCheck(cert *x509.Certificate, index int, resultsPQ *CheckResultPQ, opts *commandOpts) {
	resultImportance := index*resultImportancePerLevel + signatureImportanceLevel

	sigAlgo := cert.SignatureAlgorithm.String()

	switch cert.SignatureAlgorithm {
	case x509.MD2WithRSA, x509.MD5WithRSA:
		heap.Push(resultsPQ, &CheckResult{
			&resultImportance,
			fmt.Sprintf("HTTP CRITICAL - x509 certificate %s uses weak signature algorithm %s",
				formatCertSubject(cert), sigAlgo),
			CRITICAL,
		})

		opts.tracef("certificate check: cert %s uses weak signature algorithm %s", formatCertSubject(cert), sigAlgo)
	case x509.SHA1WithRSA:
		heap.Push(resultsPQ, &CheckResult{
			&resultImportance,
			fmt.Sprintf("HTTP WARNING - x509 certificate %s uses deprecated SHA1 signature algorithm %s",
				formatCertSubject(cert), sigAlgo),
			WARNING,
		})

		opts.tracef("certificate check: cert %s uses deprecated SHA1 signature %s", formatCertSubject(cert), sigAlgo)
	default:
		heap.Push(resultsPQ, &CheckResult{
			&resultImportance,
			fmt.Sprintf("HTTP OK - x509 certificate %s uses strong signature algorithm %s",
				formatCertSubject(cert), sigAlgo),
			OK,
		})
	}
}

// pushNotBeforeCheck verifies the certificate is not used before its validity period begins.
func pushNotBeforeCheck(cert *x509.Certificate, index int, timeLayout string, resultsPQ *CheckResultPQ, opts *commandOpts) {
	resultImportance := index*resultImportancePerLevel + notBeforeImportanceLevel

	notBefore := cert.NotBefore
	if time.Now().Before(notBefore) {
		heap.Push(resultsPQ, &CheckResult{
			&resultImportance,
			fmt.Sprintf("HTTP CRITICAL - x509 certificate %s has its validity start time in the future (valid from %s)",
				formatCertSubject(cert), notBefore.Format(timeLayout)),
			CRITICAL,
		})

		opts.tracef("certificate check: cert %s validity starts in the future: %s", formatCertSubject(cert), notBefore.Format(timeLayout))

		return
	}

	heap.Push(resultsPQ, &CheckResult{
		&resultImportance,
		fmt.Sprintf("HTTP OK - x509 certificate %s has its validity start time in the past (valid from %s)",
			formatCertSubject(cert), notBefore.Format(timeLayout)),
		OK,
	})
}
