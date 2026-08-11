package check_http

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/dustin/go-humanize"
	"github.com/kdar/factorlog"
	"github.com/sni/go-flags"
)

const (
	UNKNOWN  = 3
	CRITICAL = 2
	WARNING  = 1
	OK       = 0 //nolint:varnamelen // it is simply short
)

const (
	defaultKeepAliveSeconds             = 30
	defaultIdleConnTimeoutSeconds       = 30
	defaultExpectContinueTimeoutSeconds = 30
	hoursInDays                         = 24
	maxBufferSizeLimit                  = 100e+6 // 100 MB
	maxWaitForMax                       = 180 * time.Second
	maxConsecutive                      = 5
)

// this struct is big, order fields from big to small and avoid wasting space due to memory packing.
// govet complains otherwise.
//
//nolint:lll,staticcheck // lll:Explanations are long / staticcheck Multiple choices are allowed in parser library.
type commandOpts struct {
	log                     *factorlog.FactorLog
	certificateCritDays     *int
	Expect                  []string      // parsed version of ExpectStr
	TimeoutParsed           time.Duration // parsed version of the timeoutStr after possibly appending time unit seconds
	warningThresholdParsed  time.Duration // parsed version of the warningThreshold after possibly appending time unit seconds
	criticalThresholdParsed time.Duration // parsed version of the warningThreshold after possibly appending time unit seconds
	certificateWarnDays     int
	bufferSize              uint64
	tlsMaxVersion           uint16
	tlsMinVersion           uint16
	flags                   struct {
		Hostname                 string        `short:"H" long:"hostname"                                description:"Host name using Host headers"`
		IPAddress                string        `short:"I" long:"IP-address"                              description:"IP address or Host name"`
		Method                   string        `short:"j" long:"method"            default:"GET"         description:"Set HTTP Method"`
		URI                      string        `short:"u" long:"uri"               default:"/"           description:"URI to request"`
		ExpectStr                string        `short:"e" long:"expect"            default:""            description:"Comma-delimited list of expected HTTP response status. By default, 1XX, 2XX are OK, 3XX depends on --onredirect option, 4XX are WARNING, 5XX are CRITICAL"`
		ExpectContent            string        `short:"s" long:"string"                                  description:"String to expect in the content"`
		Base64ExpectContent      string        `          long:"base64-string"                           description:"Base64 Encoded string to expect the content"`
		UserAgent                string        `short:"A" long:"useragent"         default:"check_http"  description:"UserAgent to be sent"`
		Authorization            string        `short:"a" long:"authorization"                           description:"Pass '[username]:[password]' formatted string to be used as basic authorization header"`
		Header                   []string      `short:"k" long:"header"                                  description:"Any other tags to be sent in http header. Use multiple times for additional headers"`
		Certificate              string        `short:"C" long:"certificate"                             description:"Check certificates instead of content. Specified in mandatory days left to warn and optional days to crit with a comma: warn_days[,<crit_days>]" `
		TLSMinVersion            string        `          long:"tls-min"                                 description:"Minimum supported TLS version. Values with plus set the max tls version as well to latest version: 1.3" choice:"1.0" choice:"1.0+" choice:"1.1" choice:"1.1+" choice:"1.2" choice:"1.2+" choice:"1.3"`
		TLSMaxVersion            string        `          long:"tls-max"                                 description:"Maximum supported TLS version" choice:"1.0" choice:"1.1" choice:"1.2" choice:"1.3"`
		Proxy                    string        `          long:"proxy"                                   description:"Proxy that should be used"`
		RegexStr                 string        `short:"r" long:"regex"                                   description:"Search page for case-sensitive regex string"`
		RegexiStr                string        `short:"R" long:"regexi"                                  description:"Search page for case-insensitive regex string"`
		Onredirect               string        `short:"f" long:"onredirect"                              description:"What strategy to use when encountering a redirect. ok/warning/critical returns immediately. follow uses the new URL returned by golang HTTP client. Sticky keeps the hostname to be same after redirect, and stickyport persists the port as well." choice:"ok" choice:"warning" choice:"critical" choice:"follow" choice:"sticky" choice:"stickyport"`
		MaxBufferSize            string        `          long:"max-buffer-size"   default:"1MB"         description:"Max buffer size to read response body (max.: 100MB)"`
		TimeoutStr               string        `short:"t" long:"timeout"           default:"10"          description:"Timeout to wait for connection. If no time unit is given at the end, default of seconds is assumed"`
		WarningThresholdStr      string        `short:"w" long:"warning"           default:"30"          description:"If the request+response takes longer specified warning threshold, raises a warning. If no time unit is given at the end, default of seconds is assumed. Value is truncated to milliseconds."`
		CriticalThresholdStr     string        `short:"c" long:"critical"          default:"60"          description:"If the request+response takes longer specified critical threshold, raises a critical. If no time unit is given at the end, default of seconds is assumed. Value is truncated to milliseconds."`
		WaitForInterval          time.Duration `          long:"wait-for-interval" default:"2s"          description:"Retry interval"`
		WaitForMax               time.Duration `          long:"wait-for-max"                            description:"Time to wait for success (max.: 180s)"`
		Interim                  time.Duration `          long:"interim"           default:"1s"          description:"Interval time after successful request for consecutive mode"`
		Consecutive              int           `          long:"consecutive"       default:"1"           description:"Number of consecutive successful requests required (max.: 5)"`
		Port                     int           `short:"p" long:"port"                                    description:"Port number"`
		MaxRedirects             int           `          long:"max-redirs"                              description:"Maximum redirects before giving up on following"`
		NoDiscard                bool          `          long:"no-discard"                              description:"Raise error when the response body is larger then max-buffer-size"`
		WaitFor                  bool          `          long:"wait-for"                                description:"Retry until successful when enabled"`
		SSL                      bool          `short:"S" long:"ssl"                                     description:"Use https"`
		SNI                      bool          `          long:"sni"                                     description:"Enable SNI"`
		TCP4                     bool          `short:"4"                                                description:"Use tcp4 only"`
		TCP6                     bool          `short:"6"                                                description:"Use tcp6 only"`
		Verbose                  bool          `short:"v" long:"verbose"                                 description:"Show verbose output"`
		ShowBody                 bool          `          long:"show-body"                               description:"Print body content below status line"`
		IgnoreCertificateChain   bool          `          long:"ignore-certificate-chain"                description:"During certificate check, all certificates are checked in many aspects. Toggle this option to only check the leaf (final) certificate."`
		CheckCN                  bool          `          long:"check-cn"                                description:"Subject Common Name of leaf certificate can be checked to match hostname exactly. Common Name field is now largely unused in modern web, with Subject Alternative Name fields being more prevalent and used instead of Common Name when present. It is not checked by default, use this flag to enable it."`
		CheckSAN                 bool          `          long:"check-san"                               description:"Subject Alternative Names can be checked against the hostname. SANs contain the hostnames and IP addresses this certificate is valid for. They are ignored if the certificate is a Certificate Authority type, meaning they are used to sign other certificates and not for proving security for a hostname. It is not checked by default, use this flag to enable it."`
		IgnoreNotAfter           bool          `          long:"ignore-not-after"                        description:"Certificates are invalid after the timestamp in their NotAfter has passed. This field can be ignored with this flag."`
		IgnoreNotBefore          bool          `          long:"ignore-not-before"                       description:"Certificates are invalid before the timestamp in their NotBefore is reached. This field can be ignored with this flag."`
		IgnoreSignatureAlgorithm bool          `          long:"ignore-signature-algorithm"              description:"Some signature algorithms are deemed insecure, and are deprecated. The algorithm used can be ignored with this flag."`
	}
}

func (opts *commandOpts) tracef(format string, args ...any) {
	if !opts.flags.Verbose {
		return
	}

	if opts.log != nil {
		opts.log.Tracef(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

func (opts *commandOpts) debugf(format string, args ...any) {
	if !opts.flags.Verbose {
		return
	}

	if opts.log != nil {
		opts.log.Debugf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

func makeTLSConfig(opts *commandOpts) (conf *tls.Config) {
	//nolint:gosec // TLS check is deliberately skipped, certificate checks are done in its separate function
	conf = &tls.Config{
		InsecureSkipVerify: true,
	}

	if opts.flags.SNI {
		host, _, err := net.SplitHostPort(opts.flags.Hostname)
		if err != nil {
			host = opts.flags.Hostname
		}

		conf.ServerName = host
	}

	if opts.tlsMinVersion != 0 {
		conf.MinVersion = opts.tlsMinVersion
	}

	if opts.tlsMaxVersion != 0 {
		conf.MaxVersion = opts.tlsMaxVersion
	}

	return conf
}

// net.Dialer is for creating a TCP connection.
func makeDialer(opts *commandOpts) func(ctx context.Context, _ string, _ string) (net.Conn, error) {
	baseDialFunc := (&net.Dialer{
		Timeout:   opts.TimeoutParsed,
		KeepAlive: defaultKeepAliveSeconds * time.Second,
		DualStack: true,
	}).DialContext

	tcpMode := "tcp"
	if opts.flags.TCP4 {
		tcpMode = "tcp4"
	}

	if opts.flags.TCP6 {
		tcpMode = "tcp6"
	}

	dialFunc := func(ctx context.Context, _ string, addr string) (net.Conn, error) {
		// when a proxy is configured, the http transport passes the proxy address as addr, need to dial the proxy instead of the target
		if opts.flags.Proxy != "" && addr != "" {
			return baseDialFunc(ctx, tcpMode, addr)
		}

		// otherwise it according to  -I/-p
		// also used by the -C certificate check which calls dialFunc with an empty addr
		targetAddr := net.JoinHostPort(opts.flags.IPAddress, strconv.Itoa(opts.flags.Port))

		return baseDialFunc(ctx, tcpMode, targetAddr)
	}

	return dialFunc
}

//nolint:ireturn // it has to return an interface, http package is built that way
func makeTransport(opts *commandOpts, dialFunc func(ctx context.Context, _ string, _ string) (net.Conn, error), tlsConfig *tls.Config) (http.RoundTripper, error) {
	proxy := http.ProxyFromEnvironment

	var parsedURL *url.URL
	proxyScheme := ""

	if opts.flags.Proxy != "" {
		var err error
		parsedURL, err = url.Parse(opts.flags.Proxy)
		if err != nil {
			return nil, fmt.Errorf("Error while parsing Proxy URL. Error was: %s", err.Error())
		}

		opts.debugf("Proxy used: %q", parsedURL)

		proxyScheme = parsedURL.Scheme
		if proxyScheme == "" {
			proxyScheme = "http"
		}

		opts.debugf("Proxy is using scheme: %q", proxyScheme)

		switch proxyScheme {
		case "https":
			opts.debugf("This means a TLS connection will be established to the proxy")
		case "socks4a", "socks5h":
			opts.debugf("This means that the proxy will resolve the target hostname")
		}

		proxy = http.ProxyURL(parsedURL)
	}

	transport := &http.Transport{
		// inherited http.DefaultTransport
		Proxy:                 proxy,
		DialContext:           dialFunc,
		IdleConnTimeout:       defaultIdleConnTimeoutSeconds * time.Second,
		TLSHandshakeTimeout:   opts.TimeoutParsed,
		ExpectContinueTimeout: defaultExpectContinueTimeoutSeconds * time.Second,
		// self-customized values
		ResponseHeaderTimeout: opts.TimeoutParsed,
		TLSClientConfig:       tlsConfig,
		ForceAttemptHTTP2:     true,
	}

	if proxyScheme == "https" {
		opts.debugf("The proxy certificate will be verified")

		proxyTLSConfig := makeProxyTLSConfig(opts, parsedURL)
		transport.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dialFunc(ctx, network, addr)
			if err != nil {
				return nil, err
			}

			return tls.Client(conn, proxyTLSConfig), nil
		}
	}

	return transport, nil
}

// makeProxyTLSConfig returns a tls config that verifies the certificate of an https proxy.
func makeProxyTLSConfig(opts *commandOpts, proxyURL *url.URL) *tls.Config {
	return &tls.Config{
		ServerName: proxyURL.Hostname(),
		MinVersion: opts.tlsMinVersion,
		MaxVersion: opts.tlsMaxVersion,
	}
}

func buildRequest(ctx context.Context, opts *commandOpts) (*http.Request, error) {
	schema := "http"
	if opts.flags.SSL {
		schema = "https"
	}

	uri := fmt.Sprintf("%s://%s%s", schema, opts.flags.Hostname, opts.flags.URI)

	var buffer bytes.Buffer

	req, err := http.NewRequestWithContext(
		ctx,
		opts.flags.Method,
		uri,
		&buffer,
	)
	if err != nil {
		//nolint:wrapcheck // 3rd party source code imported
		return nil, err
	}

	if opts.flags.Authorization != "" {
		a := strings.SplitN(opts.flags.Authorization, ":", 2)
		if len(a) != 2 {
			return nil, errors.New("invalid authorization args")
		}

		req.SetBasicAuth(a[0], a[1])
	}

	req.Header.Set("User-Agent", opts.flags.UserAgent)

	// set additional headers
	for _, hdr := range opts.flags.Header {
		parts := strings.SplitN(hdr, ":", 2)
		if len(parts) == 2 {
			req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}

	return req, nil
}

type RequestMetadata struct {
	req            *http.Request
	res            *http.Response
	buffer         *capWriter
	redirectionErr *clientRedirectError
	body           string
	duration       time.Duration
}

func (m *RequestMetadata) String() string {
	if m == nil {
		return "<nil RequestMetadata>"
	}

	var status string
	if m.res != nil {
		status = m.res.Status
	} else {
		status = "(no response)"
	}

	bodyPreviewLength := 256

	var bodyPreview string

	if m.body != "" {
		if len(m.body) > bodyPreviewLength {
			bodyPreview = m.body[:bodyPreviewLength] + "..."
		} else {
			bodyPreview = m.body
		}
	} else {
		bodyPreview = "(empty)"
	}

	return fmt.Sprintf(
		"RequestMetadata{duration: %v, status: %s, body_size: %d, body_preview: %q, redirects: %v}",
		m.duration,
		status,
		m.buffer.Size(),
		bodyPreview,
		m.redirectionErr != nil,
	)
}

// Helper function to extract everything from *http.Request.
func performHTTPRequest(req *http.Request, client *http.Client, opts *commandOpts) (metadata *RequestMetadata, err error) {
	if opts.flags.Verbose {
		reqDump, _ := httputil.DumpRequest(req, true)
		opts.tracef("request:\n%s", reqDump)
	}

	start := time.Now()
	res, err := client.Do(req)
	duration := time.Since(start).Truncate(time.Millisecond)

	var redirectionErr *clientRedirectError

	//nolint:nestif // imported 3rd party source
	if err != nil {
		if urlErr, ok := errors.AsType[*url.Error](err); ok {
			if clientRedirectErr, ok := errors.AsType[*clientRedirectError](urlErr.Err); ok {
				// this is not really an error, we pack information into this error struct
				// the code acts according to the chosen follow strategy
				redirectionErr = clientRedirectErr
			} else {
				return nil, fmt.Errorf("error during request: %w", err)
			}
		} else {
			return nil, fmt.Errorf("error during request: %w", err)
		}
	}

	if opts.flags.Verbose {
		resDump, _ := httputil.DumpResponse(res, true)
		opts.tracef("response:\n%s", resDump)
	}

	var (
		buffer = &capWriter{Cap: opts.bufferSize, NoDiscard: opts.flags.NoDiscard}
		body   string
	)

	if redirectionErr == nil && res != nil && res.Body != nil {
		writtenByteCount, ioCopyErr := io.Copy(buffer, res.Body)
		defer res.Body.Close()

		if ioCopyErr != nil {
			return nil, fmt.Errorf("Error when copying request body buffer: %s , written bytes: %d", ioCopyErr.Error(), writtenByteCount)
		}

		body = string(buffer.Bytes())
	}

	// the returned err might be of type clientRedirectError
	return &RequestMetadata{
		req,
		res,
		buffer,
		redirectionErr,
		body,
		duration,
	}, nil
}

func getStatusLine(meta *RequestMetadata) string {
	return fmt.Sprintf("%s %s", meta.req.Proto, meta.res.Status)
}

func subcheckStatusLine(meta *RequestMetadata, opts *commandOpts) (matches []string, err *CheckResult) {
	if len(opts.Expect) > 0 {
		opts.tracef("subcheck: status line")

		statusLine := getStatusLine(meta)
		foundOption := ""

		for _, exceptedStatusLine := range opts.Expect {
			if strings.Contains(statusLine, exceptedStatusLine) {
				opts.tracef("response staus line: '%s' contains expected status line option: '%s'", statusLine, exceptedStatusLine)

				foundOption = exceptedStatusLine

				break
			}
		}

		if foundOption != "" {
			matches = append(matches, fmt.Sprintf(`response status line: '%s' matched option '%s'`, statusLine, foundOption))
		} else {
			return []string{}, &CheckResult{
				nil,
				fmt.Sprintf("HTTP CRITICAL - %s - response status line: '%s' does not match any of the specified options: '%v'", statusLine, statusLine, opts.Expect),
				CRITICAL,
			}
		}
	}

	return matches, nil
}

func subcheckExpectedContent(meta *RequestMetadata, opts *commandOpts) (matches []string, err *CheckResult) {
	if opts.flags.ExpectContent != "" {
		opts.tracef("subcheck: expected content")

		statusLine := getStatusLine(meta)
		if !strings.Contains(meta.body, opts.flags.ExpectContent) {
			return matches, &CheckResult{
				nil,
				fmt.Sprintf(`HTTP CRITICAL - %s - response body did not match content: %s`, statusLine, opts.flags.ExpectContent),
				CRITICAL,
			}
		}

		matches = append(matches, fmt.Sprintf("string: '%s'", opts.flags.ExpectContent))
	}

	return matches, nil
}

func subcheckBase64ExpectedContent(meta *RequestMetadata, opts *commandOpts) (matches []string, err *CheckResult) {
	if opts.flags.Base64ExpectContent != "" {
		opts.tracef("subcheck: expected content")

		statusLine := getStatusLine(meta)

		data, decodeErr := base64.StdEncoding.DecodeString(opts.flags.Base64ExpectContent)
		if decodeErr != nil {
			return matches, &CheckResult{
				nil,
				fmt.Sprintf(`HTTP CRITICAL - %s - failed to decode base64 string: %s`, statusLine, opts.flags.Base64ExpectContent),
				CRITICAL,
			}
		}

		if !bytes.Contains(meta.buffer.Bytes(), data) {
			return matches, &CheckResult{
				nil,
				fmt.Sprintf(`HTTP CRITICAL - %s - response body not matched content: %s`, statusLine, opts.flags.Base64ExpectContent),
				CRITICAL,
			}
		}

		matches = append(matches, fmt.Sprintf("base64: '%s' , string: '%s'", opts.flags.Base64ExpectContent, string(data)))
	}

	return matches, nil
}

func subcheckRegex(meta *RequestMetadata, opts *commandOpts) (matches []string, err *CheckResult) {
	if opts.flags.RegexStr != "" {
		opts.tracef("subcheck: regex")

		statusLine := getStatusLine(meta)

		regex, err := regexp.Compile(opts.flags.RegexStr)
		if err != nil {
			return matches, &CheckResult{
				nil,
				fmt.Sprintf(`HTTP UNKNOWN - %s - Could not build case sensitive regex from option: '%s'`, statusLine, opts.flags.RegexStr),
				UNKNOWN,
			}
		}

		regexMatched := regex.FindStringSubmatch(meta.body)
		if len(regexMatched) == 0 {
			return matches, &CheckResult{
				nil,
				fmt.Sprintf(`HTTP CRITICAL - %s - HTTP response body did not match regex: '%s'`, statusLine, opts.flags.RegexStr),
				CRITICAL,
			}
		}

		matches = append(matches, fmt.Sprintf("regex: '%s' , matches: '%s'", opts.flags.RegexStr, strings.Join(regexMatched, ",")))
	}

	return matches, nil
}

func subcheckRegexi(meta *RequestMetadata, opts *commandOpts) (matches []string, err *CheckResult) {
	if opts.flags.RegexiStr != "" {
		opts.tracef("subcheck: regexi")

		statusLine := getStatusLine(meta)
		// as option add (%?) case insensitive
		regex, err := regexp.Compile("(?i)" + opts.flags.RegexiStr)
		if err != nil {
			return matches, &CheckResult{
				nil,
				fmt.Sprintf(`HTTP UNKNOWN - %s - Could not build case insensitive regex from option: '%s'`, statusLine, opts.flags.RegexiStr),
				UNKNOWN,
			}
		}

		regexMatched := regex.FindStringSubmatch(meta.body)
		if len(regexMatched) == 0 {
			return matches, &CheckResult{
				nil,
				fmt.Sprintf(`HTTP CRITICAL - %s - HTTP response body did not match case insensitive regex: '%s'`, statusLine, opts.flags.RegexiStr),
				CRITICAL,
			}
		}

		matches = append(matches, fmt.Sprintf("regexi: '%s' , matches: '%s'", opts.flags.RegexiStr, strings.Join(regexMatched, ",")))
	}

	return matches, nil
}

// the request+response duration is saved onto the metadata.
// the command line arguments might have specified warning/critical thresholds to check against.
func checkDurationThresholds(meta *RequestMetadata, opts *commandOpts) (err *CheckResult) {
	opts.tracef("checking duration thresholds")

	statusLine := getStatusLine(meta)

	if opts.flags.CriticalThresholdStr != "" && opts.criticalThresholdParsed != 0 && meta.duration > opts.criticalThresholdParsed {
		return &CheckResult{
			nil,
			fmt.Sprintf("HTTP CRITICAL - %s - %d bytes in %.3f second response time (took longer than the critical threshold %.3fs) | %s",
				statusLine, meta.buffer.Size(), meta.duration.Seconds(), opts.criticalThresholdParsed.Seconds(), buildPerfdataString(opts, meta)),
			CRITICAL,
		}
	}

	if opts.flags.WarningThresholdStr != "" && opts.warningThresholdParsed != 0 && meta.duration > opts.warningThresholdParsed {
		return &CheckResult{
			nil,
			fmt.Sprintf("HTTP WARNING - %s - %d bytes in %.3f second response time (took longer than the warning threshold %.3fs) | %s",
				statusLine, meta.buffer.Size(), meta.duration.Seconds(), opts.warningThresholdParsed.Seconds(), buildPerfdataString(opts, meta)),
			WARNING,
		}
	}

	return nil
}

type clientRedirectError struct {
	redirectedReq *http.Request
	followOption  string
	originalHost  string
	originalPort  int
	stopRedirect  bool
}

func (e *clientRedirectError) Error() string {
	str := fmt.Sprintf("clientRedirectHandlerError, this value encapsulates the follow command line option: '%s' .", e.followOption)

	switch e.followOption {
	case "":
		str += "Follow option is not specified. This means following is not allowed."
	case "follow":
		str += "This uses the default behavior of go standard http package for redirections."
	case "ok":
		str += "This means that any redirection is an OK result."
	case "warning":
		str += "This means that any redirection is a WARNING result."
	case "critical":
		str += "This means that any redirection is a CRITICAL result."
	case "sticky":
		str += "This means that redirections are allowed, but the hostname/IP and the port is forced to stay the same."
	case "stickyport":
		str += "This means that redirections are allowed, but the hostname/IP and the port is forced to stay the same."
	}

	return str
}

func clientRedirectErrorHandler(err clientRedirectError, meta *RequestMetadata, opts *commandOpts) (checkResult *CheckResult, nextReq *http.Request) {
	statusLine := getStatusLine(meta)

	switch err.followOption {
	case "":
		return nil, nil
	case "follow":
		log.Panicf("This option should have returned nil and continued redirection in redirection handler.")

		return nil, nil
	// HTTP OK - 302 Found - 215 bytes in 0.045 second response time |time=0.045s size=215B
	case "ok":
		return &CheckResult{
			nil,
			fmt.Sprintf("HTTP OK - %s - %d bytes in %.3f second response time | %s",
				statusLine, meta.res.ContentLength, meta.duration.Seconds(), buildPerfdataString(opts, meta)),
			OK,
		}, nil
	case "warning":
		return &CheckResult{
			nil,
			fmt.Sprintf("HTTP WARNING - %s - %d bytes in %.3f second response time | %s",
				statusLine, meta.res.ContentLength, meta.duration.Seconds(), buildPerfdataString(opts, meta)),
			WARNING,
		}, nil
	case "critical":
		return &CheckResult{
			nil,
			fmt.Sprintf("HTTP CRITICAL - %s - %d bytes in %.3f second response time | %s",
				statusLine, meta.res.ContentLength, meta.duration.Seconds(), buildPerfdataString(opts, meta)),
			CRITICAL,
		}, nil
	case "sticky", "stickyport":
		nextReq = err.redirectedReq

		// http.Request ignores req.URL.Host is set

		var origHost, origPortStr string

		_, _, splitErr := net.SplitHostPort(err.originalHost)
		if splitErr == nil {
			origHost, origPortStr, _ = net.SplitHostPort(err.originalHost)
			if origPortStr == "" {
				// fallback to opts.Port logic
				if opts.flags.SSL {
					origPortStr = "443"
				} else {
					origPortStr = "80"
				}
			}

			switch err.followOption {
			case "sticky":
				// sticky: keep original host, follow redirect port
				nextReq.URL.Host = net.JoinHostPort(origHost, nextReq.URL.Port())
			case "stickyport":
				// stickyport: keep both host and port
				nextReq.URL.Host = net.JoinHostPort(origHost, origPortStr)
			}
		}

		nextReq.Host = nextReq.URL.Hostname()

		return nil, nextReq
	default:
		return &CheckResult{
			nil,
			fmt.Sprintf("HTTP UNKNOWN - %s - Unknown follow strategy: %s", statusLine, err.followOption),
			0,
		}, nil
	}
}

func buildPerfdataString(opts *commandOpts, meta *RequestMetadata) string {
	durationStr := strconv.FormatFloat(meta.duration.Seconds(), 'f', 3, 64)

	var warnThresholdStr string
	if opts.flags.WarningThresholdStr != "" && opts.warningThresholdParsed != 0 {
		warnThresholdStr = strconv.FormatFloat(opts.warningThresholdParsed.Seconds(), 'f', 3, 64)
	}

	var criticalThresholdStr string
	if opts.flags.CriticalThresholdStr != "" && opts.criticalThresholdParsed != 0 {
		criticalThresholdStr = strconv.FormatFloat(opts.criticalThresholdParsed.Seconds(), 'f', 3, 64)
	}

	return fmt.Sprintf(
		`time=%ss;%s;%s;0; size=%dB;;;0;`,
		durationStr,
		warnThresholdStr,
		criticalThresholdStr,
		meta.buffer.Size(),
	)
}

// if this function does not return an error, the redirection can continue
// The arguments req and via are the upcoming request and the requests made already, oldest first.
// This function is used to continue following, or encapsulate the follow strategy in an custom error type.
func clientRedirectHandler(req *http.Request, via []*http.Request, opts *commandOpts) (err error) {
	clientHandlerErr := &clientRedirectError{
		followOption:  opts.flags.Onredirect,
		redirectedReq: req,
	}
	if len(via) > 0 {
		clientHandlerErr.originalHost = via[0].URL.Host
		if clientHandlerErr.originalHost == "" {
			clientHandlerErr.originalHost = via[0].Host
		}
	} else {
		clientHandlerErr.originalHost = req.URL.Host // fallback
	}

	clientHandlerErr.originalPort = opts.flags.Port

	switch opts.flags.Onredirect {
	case "":
		// following is not enabled by default
		clientHandlerErr.stopRedirect = true
	case "follow":
		return nil
	case "ok", "warning", "critical", "sticky", "stickyport":
	default:
		return fmt.Errorf("Unknown/Unsupported follow option: %s", opts.flags.Onredirect)
	}

	return clientHandlerErr
}

// Naemon-Like function that returns naemon errors, handles redirections, checks body content.
//
//nolint:funlen // splitting the function more would be worse
func request(ctx context.Context, client *http.Client, opts *commandOpts) (okMsg string, result *CheckResult) {
	req, err := buildRequest(ctx, opts)
	if err != nil {
		return "", &CheckResult{
			nil,
			fmt.Sprintf("Error in building request: %v", err),
			UNKNOWN,
		}
	}

	var (
		meta    *RequestMetadata
		nextReq *http.Request
	)
	// first request is not a redirection , second is the first redirection
	redirectionCount := -1
	for req != nil {
		if redirectionCount > opts.flags.MaxRedirects {
			return "", &CheckResult{
				nil,
				"HTTP CRITICAL - Max redirections reached",
				CRITICAL,
			}
		}

		meta, err = performHTTPRequest(req, client, opts)
		if err != nil {
			return "", &CheckResult{
				nil,
				fmt.Sprintf("HTTP CRITICAL - Error when performing request: %s", err),
				CRITICAL,
			}
		}

		if meta.redirectionErr != nil {
			result, nextReq = clientRedirectErrorHandler(*meta.redirectionErr, meta, opts)
		}

		req = nextReq
		redirectionCount++
	}

	// redirection might have given us a check result
	// we should return this immediately
	if result != nil {
		return "", result
	}

	// sanity check
	if meta == nil {
		return "", &CheckResult{
			nil,
			"HTTP CRITICAL - Error when performing request",
			CRITICAL,
		}
	}

	opts.tracef("request metadata: %v", meta)

	var reqErr *CheckResult

	matches := []string{}

	// Need to mimic the order HTTP checks are made
	// Status line check (--expect)
	// Header string check (--header-string)
	// Content string check (--string)
	// Case-sensitive regex (--regex)
	// Case-insensitive regex (--eregi)
	// Page size check

	// erroneus status codes check

	matchesStatusLine, reqErr := subcheckStatusLine(meta, opts)
	if reqErr != nil {
		reqErr.msg += " | " + buildPerfdataString(opts, meta)

		return "", reqErr
	}

	matches = append(matches, matchesStatusLine...)

	// Header string checks are not yet implemented

	matchesContent, reqErr := subcheckExpectedContent(meta, opts)
	if reqErr != nil {
		reqErr.msg += " | " + buildPerfdataString(opts, meta)

		return "", reqErr
	}

	matches = append(matches, matchesContent...)

	matchesBase64Content, reqErr := subcheckBase64ExpectedContent(meta, opts)
	if reqErr != nil {
		reqErr.msg += " | " + buildPerfdataString(opts, meta)

		return "", reqErr
	}

	matches = append(matches, matchesBase64Content...)

	matchesRegex, reqErr := subcheckRegex(meta, opts)
	if reqErr != nil {
		reqErr.msg += " | " + buildPerfdataString(opts, meta)

		return "", reqErr
	}

	matches = append(matches, matchesRegex...)

	matchesRegexi, reqErr := subcheckRegexi(meta, opts)
	if reqErr != nil {
		reqErr.msg += " | " + buildPerfdataString(opts, meta)

		return "", reqErr
	}

	matches = append(matches, matchesRegexi...)

	matchesOutputStr := ""
	if len(matches) > 0 {
		matchesOutputStr = fmt.Sprintf("Response body matched: [%s] - ", strings.Join(matches, ", "))
	}

	// Page size check is not yet implemented

	reqErr = checkDurationThresholds(meta, opts)
	if reqErr != nil {
		return "", reqErr
	}

	reqErr = handleErroneousHTTPReturnCodes(meta.res, opts, meta)
	if reqErr != nil {
		reqErr.msg += " | " + buildPerfdataString(opts, meta)

		return "", reqErr
	}

	statusLine := getStatusLine(meta)

	_, err = meta.buffer.Write([]byte(statusLine + "\r\n\r\n"))
	if err != nil {
		return "", &CheckResult{
			nil,
			fmt.Sprintf("HTTP UNKNOWN - Error when writing statusLine to buffer: %s", err),
			UNKNOWN,
		}
	}

	err = meta.res.Header.Write(meta.buffer)
	if err != nil {
		return "", &CheckResult{
			nil,
			fmt.Sprintf("HTTP UNKNOWN - Error when writing header to buffer: %s", err),
			UNKNOWN,
		}
	}

	showBodyStr := ""
	if opts.flags.ShowBody {
		showBodyStr = "\n" + meta.body
	}

	okMsg = fmt.Sprintf(
		`HTTP OK - %s - %s %d bytes in %.3fs response time | %s %s`,
		statusLine, matchesOutputStr, meta.buffer.Size(), meta.duration.Seconds(),
		buildPerfdataString(opts, meta), showBodyStr,
	)

	return okMsg, nil
}

// If the HTTP status code is erroneus, return a non-nil err.
func handleErroneousHTTPReturnCodes(res *http.Response, opts *commandOpts, meta *RequestMetadata) (err *CheckResult) {
	if len(opts.Expect) > 0 {
		// if a statusLine is expected, HTTP error code checks are disabled
		return nil
	}

	// 1xx (Informational)    → STATE_OK (success)
	// 2xx (Success)          → STATE_OK (success)
	// 3xx (Redirection)      → Depends on --onredirect setting
	// 4xx (Client Error)     → STATE_WARNING
	// 5xx (Server Error)     → STATE_CRITICAL
	// < 100 or >= 600        → STATE_CRITICAL (invalid status code)

	opts.tracef("checking for erroneus HTTP return codes")

	statusLine := fmt.Sprintf("%s %s", meta.res.Proto, meta.res.Status)
	// Between 400 and 500
	if http.StatusBadRequest <= res.StatusCode && res.StatusCode < http.StatusInternalServerError {
		return &CheckResult{
			nil,
			fmt.Sprintf("HTTP WARNING - %s - Invalid HTTP response received", statusLine),
			WARNING,
		}
	}

	// Above 500
	if http.StatusInternalServerError <= res.StatusCode {
		return &CheckResult{
			nil,
			fmt.Sprintf("HTTP CRITICAL - %s - Invalid HTTP response received", statusLine),
			CRITICAL,
		}
	}

	return nil
}

const (
	// if this check is used in snclient, try to get its logger from the context.
	// prefer snclients logger over the default logger if its present.
	// snclient packs the logger in the context using this key.
	snclientLoggerContextKey string = "github.com/consol-monitoring/snclient/pkg/utils.Logger"
)

//nolint:funlen,maintidx,gocyclo //the main function has a lot of argument parsing
func Check(ctx context.Context, output io.Writer, osArgs []string) int {
	opts := commandOpts{}
	psr := flags.NewParser(&opts.flags, flags.HelpFlag|flags.PassDoubleDash) // default flags without flags.PrintErrors
	psr.Name = "check_http"

	_, err := psr.ParseArgs(osArgs)
	if err != nil {
		fmt.Fprintf(output, "%s\n", err.Error())

		return UNKNOWN
	}

	var loggerCastOk bool

	opts.log, loggerCastOk = ctx.Value(snclientLoggerContextKey).(*factorlog.FactorLog)
	opts.tracef("extracting logger from context using snclient logger specific key result: %t", loggerCastOk)

	bufferSize, err := humanize.ParseBytes(opts.flags.MaxBufferSize)
	if err != nil {
		fmt.Fprintf(output, "Could not parse max-buffer-size: %v\n", err)

		return UNKNOWN
	}
	if bufferSize > maxBufferSizeLimit {
		fmt.Fprintf(output, "Max-buffer-size exceeds the limit: %d\n", bufferSize)

		return UNKNOWN
	}
	opts.bufferSize = bufferSize

	if opts.flags.WaitFor && opts.flags.WaitForMax == 0 {
		fmt.Fprintf(output, "wait-for-max is required when wait-for is enabled\n")

		return UNKNOWN
	}
	if opts.flags.WaitForMax > maxWaitForMax {
		fmt.Fprintf(output, "wait-for-max exceeds the limit: %s (limit: %s)\n", opts.flags.WaitForMax, maxWaitForMax)

		return UNKNOWN
	}

	if opts.flags.Consecutive > maxConsecutive {
		fmt.Fprintf(output, "consecutive exceeds the limit: %d (limit: %d)\n", opts.flags.Consecutive, maxConsecutive)

		return UNKNOWN
	}

	if opts.flags.ExpectStr != "" {
		opts.Expect = strings.Split(opts.flags.ExpectStr, ",")
	}

	if opts.flags.ExpectContent != "" && opts.flags.Base64ExpectContent != "" {
		fmt.Fprintf(output, "Both string and base64-string are specified\n")

		return UNKNOWN
	}

	if opts.flags.Base64ExpectContent != "" {
		_, decodeErr := base64.StdEncoding.DecodeString(opts.flags.Base64ExpectContent)
		if decodeErr != nil {
			fmt.Fprintf(output, "Failed decode base64-string: %v\n", decodeErr)

			return UNKNOWN
		}
	}

	if opts.flags.TCP4 && opts.flags.TCP6 {
		fmt.Fprintf(output, "Both tcp4 and tcp6 are specified\n")

		return UNKNOWN
	}

	if opts.flags.SNI && opts.flags.Hostname == "" {
		fmt.Fprintf(output, "hostname is required when use sni\n")

		return UNKNOWN
	}

	if opts.flags.Hostname == "" && opts.flags.IPAddress == "" {
		fmt.Fprintf(output, "Specify either hostname or ipaddress\n")

		return UNKNOWN
	}

	if opts.flags.Hostname == "" {
		opts.flags.Hostname = opts.flags.IPAddress
	}

	if opts.flags.IPAddress == "" {
		host, _, splitErr := net.SplitHostPort(opts.flags.Hostname)
		if splitErr != nil {
			opts.flags.IPAddress = opts.flags.Hostname
		} else {
			opts.flags.IPAddress = host
		}
	}

	if opts.flags.Port == 0 {
		_, port, splitErr := net.SplitHostPort(opts.flags.Hostname)
		if splitErr == nil {
			p, _ := strconv.Atoi(port)
			// skip error check OK
			opts.flags.Port = p
		}
	}

	// automatically enable SSL, this is the behavior of monitoring-plugins check_http
	if opts.flags.Certificate != "" {
		if !opts.flags.SSL {
			opts.flags.SSL = true
		}
	}

	if opts.flags.Port == 0 {
		if opts.flags.SSL {
			opts.flags.Port = 443
		} else {
			opts.flags.Port = 80
		}
	}

	if opts.flags.URI == "" {
		opts.flags.URI = "/"
	}

	if opts.flags.MaxRedirects == 0 {
		opts.flags.MaxRedirects = 15
	}

	timeoutStrLastRune, _ := utf8.DecodeLastRuneInString(opts.flags.TimeoutStr)
	if unicode.IsDigit(timeoutStrLastRune) {
		opts.flags.TimeoutStr += "s"
	}

	var timeoutParseErr error

	opts.TimeoutParsed, timeoutParseErr = time.ParseDuration(opts.flags.TimeoutStr)
	if timeoutParseErr != nil {
		fmt.Fprintf(output, "Error parsing timeoutStr: %q , %s", opts.flags.TimeoutStr, timeoutParseErr.Error())

		return UNKNOWN
	}

	warningThresholdLastRune, _ := utf8.DecodeLastRuneInString(opts.flags.WarningThresholdStr)
	if unicode.IsDigit(warningThresholdLastRune) {
		opts.flags.WarningThresholdStr += "s"
	}

	var warningThresholdParseErr error

	opts.warningThresholdParsed, warningThresholdParseErr = time.ParseDuration(opts.flags.WarningThresholdStr)
	if warningThresholdParseErr != nil {
		fmt.Fprintf(output, "Error parsing warningThresholdStr: %q , %s", opts.flags.WarningThresholdStr, warningThresholdParseErr.Error())

		return UNKNOWN
	}

	opts.warningThresholdParsed = opts.warningThresholdParsed.Truncate(time.Millisecond)

	criticalThresholdLastRune, _ := utf8.DecodeLastRuneInString(opts.flags.CriticalThresholdStr)
	if unicode.IsDigit(criticalThresholdLastRune) {
		opts.flags.CriticalThresholdStr += "s"
	}

	var criticalThresholdParseErr error

	opts.criticalThresholdParsed, criticalThresholdParseErr = time.ParseDuration(opts.flags.CriticalThresholdStr)
	if criticalThresholdParseErr != nil {
		fmt.Fprintf(output, "Error parsing criticalThresholdStr: %q , %s", opts.flags.CriticalThresholdStr, criticalThresholdParseErr.Error())

		return UNKNOWN
	}

	opts.criticalThresholdParsed = opts.criticalThresholdParsed.Truncate(time.Millisecond)

	switch opts.flags.TLSMinVersion {
	// argument parser only accepts these values as valid
	case "1.0":
		opts.tlsMinVersion = tls.VersionTLS10
	case "1.0+":
		opts.tlsMinVersion = tls.VersionTLS10
		opts.tlsMaxVersion = tls.VersionTLS13
	case "1.1":
		opts.tlsMinVersion = tls.VersionTLS11
	case "1.1+":
		opts.tlsMinVersion = tls.VersionTLS11
		opts.tlsMaxVersion = tls.VersionTLS13
	case "1.2":
		opts.tlsMinVersion = tls.VersionTLS12
	case "1.2+":
		opts.tlsMinVersion = tls.VersionTLS12
		opts.tlsMaxVersion = tls.VersionTLS13
	case "1.3":
		opts.tlsMinVersion = tls.VersionTLS13
	}

	switch opts.flags.TLSMaxVersion {
	// argument parser only accepts these values as valid
	case "1.0":
		opts.tlsMaxVersion = tls.VersionTLS10
	case "1.1":
		opts.tlsMaxVersion = tls.VersionTLS11
	case "1.2":
		opts.tlsMaxVersion = tls.VersionTLS12
	case "1.3":
		opts.tlsMaxVersion = tls.VersionTLS13
	}

	if opts.tlsMinVersion != 0 && opts.tlsMaxVersion != 0 && opts.tlsMinVersion > opts.tlsMaxVersion {
		fmt.Fprintf(output, "TLS min version value is higher than TLS max version value, check your arguments.\n")

		return UNKNOWN
	}

	//nolint:nestif // imported 3rd party source
	if opts.flags.Certificate != "" {
		splits := strings.SplitN(opts.flags.Certificate, ",", 2)

		parseDays := func(str string) (int, error) {
			if str == "" {
				return 0, nil
			}

			parsedInt, parseErr := convert.Int64E(str)
			if parseErr != nil {
				return 0, fmt.Errorf("int parse error: %w", parseErr)
			}

			if parsedInt < 0 {
				return 0, errors.New("days remaining cannot be a negative value")
			}

			return int(parsedInt), nil
		}

		warnDays, parseWarnErr := parseDays(splits[0])
		if parseWarnErr != nil {
			fmt.Fprintf(output, "Certificate check warning days could not be parsed: %s.\n", parseWarnErr.Error())

			return UNKNOWN
		}

		opts.certificateWarnDays = warnDays

		if len(splits) == 2 {
			critDays, parseCritErr := parseDays(splits[1])
			if parseCritErr != nil {
				fmt.Fprintf(output, "Certificate check critical days could not be parsed: %s.\n", parseCritErr.Error())

				return UNKNOWN
			}

			if critDays > warnDays {
				fmt.Fprintf(output, "Certificate expiration date check: critical days cannot be higher than warning days.\n")

				return UNKNOWN
			}

			opts.certificateCritDays = &critDays
		}
	}

	// Build shared TLS config and dialer
	tlsConfig := makeTLSConfig(&opts)
	dialFunc := makeDialer(&opts)

	// If certificate check is enabled, perform certificate validation and return
	if opts.flags.Certificate != "" {
		timeout := opts.TimeoutParsed

		certCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		certResult := checkCertificate(certCtx, &opts, dialFunc, tlsConfig)
		fmt.Fprintf(output, "%s\n", certResult.Error())

		return certResult.Code()
	}

	transport, err := makeTransport(&opts, dialFunc, tlsConfig)
	if err != nil {
		fmt.Fprintf(output, "Error in http configuration: %s\n", err.Error())

		return UNKNOWN
	}

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return clientRedirectHandler(req, via, &opts)
		},
		Timeout: opts.TimeoutParsed,
	}

	timeout := opts.TimeoutParsed
	if opts.flags.WaitForMax > 0 {
		timeout = opts.flags.WaitForMax
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	requestNum := 0

	if opts.flags.WaitFor {
		consecutive := opts.flags.Consecutive - 1

		for ctx.Err() == nil {
			requestNum++
			okMsg, reqErr := request(ctx, client, &opts)

			interval := opts.flags.Interim

			switch {
			case reqErr == nil && consecutive <= 0:
				opts.tracef("request[%d]: %s", requestNum, okMsg)

				fmt.Fprint(output, okMsg)

				return OK
			case reqErr == nil:
				consecutive--

				opts.tracef("request[%d]: %s", requestNum, okMsg)
			default:
				interval = opts.flags.WaitForInterval

				consecutive = opts.flags.Consecutive - 1

				opts.tracef("request[%d]: %s", requestNum, reqErr.Error())
			}

			select {
			case <-ctx.Done():
			case <-time.After(interval):
			}
		}

		fmt.Fprint(output, "Give up waiting for success\n")

		return UNKNOWN
	}

	consecutive := opts.flags.Consecutive - 1

	var reqErr *CheckResult

requestLoop:
	for ctx.Err() == nil {
		var okMsg string

		requestNum++

		okMsg, reqErr = request(ctx, client, &opts)
		switch {
		case reqErr == nil && consecutive <= 0:
			opts.tracef("request[%d]: %s", requestNum, okMsg)

			fmt.Fprint(output, okMsg)

			return OK
		case reqErr == nil:
			consecutive--

			opts.tracef("request[%d]: %s", requestNum, okMsg)
		default:
			break requestLoop
		}

		select {
		case <-ctx.Done():
		case <-time.After(opts.flags.Interim):
		}
	}

	fmt.Fprint(output, reqErr.Error())

	return reqErr.Code()
}
