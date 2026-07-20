// nolint:ALL
package check_dns

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/consol-monitoring/snclient/pkg/utils"
	"github.com/mackerelio/checkers"
	"github.com/miekg/dns"
	"github.com/sni/go-flags"
)

func Check(ctx context.Context, output io.Writer, args []string) int {
	opts, err := parseArgs(args)
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			fmt.Fprint(output, err.Error())
			return int(checkers.OK)
		}
		fmt.Fprintf(output, "UNKNOWN - %s", err.Error())
		return int(checkers.UNKNOWN)
	}

	ckr := opts.run(ctx)
	fmt.Fprintf(output, "%s - %s", ckr.Status, strings.TrimSpace(ckr.Message))

	return int(ckr.Status)
}

// adopted from https://raw.githubusercontent.com/mackerelio/go-check-plugins/master/check-dns/lib/
// Apache-2.0 license
type dnsOpts struct {
	Host            string   `short:"H" long:"host" required:"true" description:"The name or address you want to query"`
	Servers         []string `short:"s" long:"server" description:"DNS servers to use for the lookup. This can be specified multiple times."`
	Port            int      `short:"p" long:"port" default:"53" description:"Port number you want to use"`
	QueryType       string   `short:"q" long:"querytype" default:"A" description:"DNS record query type"`
	Norec           bool     `long:"norec" description:"Clears the Recursion Desired flag, DNS server answers only from its authoritative data or cache, does not ask other nameservers."`
	ExpectedString  []string `short:"e" long:"expected-string" description:"IP-ADDRESS string you expect the DNS server to return. If multiple IP-ADDRESS are returned at once, you have to specify whole string"`
	SearchPaths     []string `long:"search-path" description:"Search paths to add to domains before sending a DNS query. This can be specified multiple times."`
	ResolvConfFile  string   `long:"resolv-conf-file" default:"/etc/resolv.conf" description:"Path to the resolv.conf file to use. Is not used in Windows."`
	Verbose         bool     `short:"v" long:"vv" long:"vvv" long:"verbose" description:"Show verbose output."`
	WarningTimeout  *int     `short:"w" long:"warning" description:"Return warning if elapsed time to get a successful DNS query exceeds this value in seconds. Default is off."`
	CriticalTimeout *int     `short:"c" long:"critical" description:"Return critical if elapsed time to get a successful DNS query exceeds this value in seconds. Default ist off."`
	Timeout         int      `short:"t" long:"timeout" default:"30" description:"Global timeout in seconds. Exit early and return unknown if elapsed time to get a successful DNS query exceeds this value."`
	QueryTimeout    int      `short:"T" long:"query-timeout" default:"5" description:"Timeout for each single DNS query in seconds. If exceeded, the next query is tried instead of exiting."`
}

func parseArgs(args []string) (*dnsOpts, error) {
	opts := &dnsOpts{}
	psr := flags.NewParser(opts, flags.HelpFlag|flags.PassDoubleDash) // default flags without flags.PrintErrors
	psr.Name = "check_dns"
	_, err := psr.ParseArgs(args)
	if err != nil {
		return opts, err
	}

	return opts, opts.validate()
}

func (opts *dnsOpts) validate() error {
	if strings.TrimSpace(opts.Host) == "" {
		return fmt.Errorf("host must not be empty")
	}
	if strings.TrimSpace(opts.QueryType) == "" {
		return fmt.Errorf("query type must not be empty")
	}
	if opts.Port < 1 || opts.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got: %d", opts.Port)
	}
	if opts.Timeout <= 0 {
		return fmt.Errorf("timeout must be a positive number of seconds, got: %d", opts.Timeout)
	}
	if opts.QueryTimeout <= 0 {
		return fmt.Errorf("query timeout must be a positive number of seconds, got: %d", opts.QueryTimeout)
	}
	if opts.WarningTimeout != nil && *opts.WarningTimeout < 0 {
		return fmt.Errorf("warning threshold must not be negative, got: %d", *opts.WarningTimeout)
	}
	if opts.CriticalTimeout != nil && *opts.CriticalTimeout < 0 {
		return fmt.Errorf("critical threshold must not be negative, got: %d", *opts.CriticalTimeout)
	}
	if opts.WarningTimeout != nil && opts.CriticalTimeout != nil && *opts.WarningTimeout > *opts.CriticalTimeout {
		return fmt.Errorf("warning threshold (%d) must not be higher than the critical threshold (%d)", *opts.WarningTimeout, *opts.CriticalTimeout)
	}
	for _, expected := range opts.ExpectedString {
		if strings.TrimSpace(expected) == "" {
			return fmt.Errorf("expected string must not be empty")
		}
	}

	return nil
}

func (opts *dnsOpts) run(ctx context.Context) *checkers.Checker {
	var err error
	var clientConfig *dns.ClientConfig

	logger := utils.LoggerFromContext(ctx)
	tryLogTrace := func(logline string) {
		if logger != nil && opts.Verbose {
			logger.Trace(logline)
		}
	}
	tryLogDebug := func(logline string) {
		if logger != nil && opts.Verbose {
			logger.Debug(logline)
		}
	}

	switch runtime.GOOS {
	case "linux", "darwin", "freebsd":
		clientConfig, err = dns.ClientConfigFromFile(opts.ResolvConfFile)
		if err != nil {
			return checkers.Critical(err.Error())
		}
	default:
	}

	var nameservers []string
	if len(opts.Servers) > 0 {
		nameservers = opts.Servers
	} else {
		nameservers, err = adapterAddress(clientConfig)
		if err != nil {
			return checkers.Critical(err.Error())
		}
	}
	for i := range nameservers {
		nameservers[i] = net.JoinHostPort(nameservers[i], strconv.Itoa(opts.Port))
	}
	if logger != nil && opts.Verbose {
		logger.Tracef("DNS nameservers: %v ", nameservers)
	}

	var searchPaths []string
	if len(opts.SearchPaths) > 0 {
		searchPaths = opts.SearchPaths
	} else {
		searchPaths = getSearchPaths(clientConfig)
	}
	if logger != nil && opts.Verbose {
		logger.Tracef("DNS search paths: %v ", searchPaths)
	}

	var hostCandidates []string
	originalHost := opts.Host
	if dns.IsFqdn(originalHost) {
		hostCandidates = append(hostCandidates, dns.Fqdn(originalHost))
	} else {
		for _, searchPath := range searchPaths {
			candidate := dns.Fqdn(originalHost + "." + searchPath)
			hostCandidates = append(hostCandidates, candidate)
		}
		// try the bare host as FQDN as well without a searchPath
		hostCandidates = append(hostCandidates, dns.Fqdn(originalHost))
	}
	if logger != nil && opts.Verbose {
		logger.Tracef("DNS host candidates: %v ", hostCandidates)
	}

	queryType, ok := dns.StringToType[strings.ToUpper(opts.QueryType)]
	if !ok {
		return checkers.Critical(fmt.Sprintf("%s is an invalid query type", opts.QueryType))
	}

	// Timeout is a builtin cumulative timeout for dial, write and read, it is applied to every single Exchange i.e. DNS query.
	c := &dns.Client{Timeout: time.Duration(opts.QueryTimeout) * time.Second}

	var r *dns.Msg
	var duration time.Duration

	var successfulNameserver string
	var successfulDuration time.Duration
	var successfulHost string

	queryDNSChan := make(chan bool, 1)
	queryDNSSuccessful := false

	emptyResults := make([]emptyResult, 0)
	recordEmptyResult := func(hostCandidate, nameserver, reason string) {
		emptyResults = append(emptyResults, emptyResult{
			host:       hostCandidate,
			nameserver: nameserver,
			reason:     reason,
		})
	}

	queryDNS := func() {
		gotAnswer := false

		for _, hostCandidate := range hostCandidates {
			for _, nameserver := range nameservers {
				message := &dns.Msg{
					MsgHdr: dns.MsgHdr{
						RecursionDesired: !opts.Norec,
						Opcode:           dns.OpcodeQuery,
					},
					Question: []dns.Question{
						{
							Name:   hostCandidate,
							Qtype:  queryType,
							Qclass: dns.StringToClass["IN"],
						},
					},
				}
				message.Id = dns.Id()

				r, duration, err = c.Exchange(message, nameserver)

				if err == nil {
					if len(r.Answer) == 0 {
						reason := emptyResultReason(r.Rcode)
						recordEmptyResult(hostCandidate, nameserver, reason)
						tryLogTrace(fmt.Sprintf("DNS query returned empty result (%s), continuing to next combination, host: %s, nameserver: %s, duration: %dms",
							reason, hostCandidate, nameserver, duration.Milliseconds()))

						continue
					}

					successfulNameserver = nameserver
					successfulHost = hostCandidate
					successfulDuration = duration
					gotAnswer = true
					tryLogDebug(fmt.Sprintf("successfully queried DNS, host: %s, nameserver: %s, duration: %dms", successfulHost, successfulNameserver, successfulDuration.Milliseconds()))

					break
				}

				recordEmptyResult(hostCandidate, nameserver, queryFailedReason(err))
				tryLogTrace(fmt.Sprintf("DNS query failed, host: %s, nameserver: %s, duration: %dms, error: %v", hostCandidate, nameserver, duration.Milliseconds(), err))
			}

			if gotAnswer {
				break
			}
		}

		queryDNSChan <- gotAnswer
	}

	queriesBeginTimestamp := time.Now()
	go queryDNS()

	select {
	case <-time.After(time.Duration(opts.Timeout) * time.Second):
		return checkers.Unknown(fmt.Sprintf("Failed to get a result after %d seconds", opts.Timeout))
	case queryDNSSuccessful = <-queryDNSChan:
	}
	queriesEndTimestamp := time.Now()
	queriesDuration := queriesEndTimestamp.Sub(queriesBeginTimestamp)

	if !queryDNSSuccessful || r == nil {
		return checkers.Critical(emptyResultsMessage(originalHost, nameservers, emptyResults))
	}

	checkSt := checkers.OK

	escalateStatus := func(newStatus checkers.Status) {
		if newStatus > checkSt {
			checkSt = newStatus
			tryLogTrace(fmt.Sprintf("status escalated to %s", checkSt.String()))
		}
	}

	switch {
	case opts.CriticalTimeout != nil && queriesDuration.Seconds() > float64(*opts.CriticalTimeout):
		tryLogTrace(fmt.Sprintf("DNS query took %f seconds, which is higher than the critical threshold: %d", queriesDuration.Seconds(), *opts.CriticalTimeout))
		escalateStatus(checkers.CRITICAL)
	case opts.WarningTimeout != nil && queriesDuration.Seconds() > float64(*opts.WarningTimeout):
		tryLogTrace(fmt.Sprintf("DNS query took %f seconds, which is higher than the warning threshold: %d", queriesDuration.Seconds(), *opts.WarningTimeout))
		escalateStatus(checkers.WARNING)
	default:
		tryLogTrace(fmt.Sprintf("DNS query took %f seconds, and it is lower than (if specified) warning threshold: %v and critical threshold: %v",
			queriesDuration.Seconds(), opts.WarningTimeout, opts.CriticalTimeout))
	}

	answersWithoutHeaders := make([]string, 0)
	answerTypes := make([]string, 0)
	for _, answer := range r.Answer {
		answerWithoutHeader, answerType, err := dnsAnswer(answer)
		if err != nil {
			return checkers.Critical(err.Error())
		}
		answersWithoutHeaders = append(answersWithoutHeaders, answerWithoutHeader)
		answerTypes = append(answerTypes, answerType)
	}

	// Special handling of returned DNS addresses VS expected DNS addresses, with set comparisons
	/**
	  if DNS server return 1.1.1.1, 2.2.2.2
		1: -e 1.1.1.1 -e 2.2.2.2            -> OK
		2: -e 1.1.1.1 -e 2.2.2.2 -e 3.3.3.3 -> WARNING
		3: -e 1.1.1.1                       -> WARNING
		4: -e 1.1.1.1 -e 3.3.3.3            -> WARNING
		5: -e 3.3.3.3                       -> CRITICAL
		6: -e 3.3.3.3 -e 4.4.4.4 -e 5.5.5.5 -> CRITICAL
	**/
	if len(opts.ExpectedString) != 0 {
		supportedQueryType := map[string]int{"A": 1, "AAAA": 1, "MX": 1, "CNAME": 1}
		_, ok := supportedQueryType[strings.ToUpper(opts.QueryType)]
		if !ok {
			return checkers.Critical(fmt.Sprintf("%s is not a supported query type. Only A, AAAA, MX, CNAME are supported query types.", opts.QueryType))
		}

		expectedStringsContainOneAnswerAddress := slices.ContainsFunc(opts.ExpectedString, func(ex string) bool {
			return slices.Contains(answersWithoutHeaders, ex)
		})

		answerCopy := slices.Clone(answersWithoutHeaders)
		expectedCopy := slices.Clone(opts.ExpectedString)
		slices.Sort(answerCopy)
		slices.Sort(expectedCopy)
		expectedStringsAndAnswersAreSame := slices.Equal(answerCopy, expectedCopy)

		switch {
		case expectedStringsAndAnswersAreSame:
			checkSt = checkers.OK
			tryLogTrace(fmt.Sprintf("Expected strings: %v and strings from the DNS answer: %v , are the same",
				opts.ExpectedString, answerCopy))
		case expectedStringsContainOneAnswerAddress:
			tryLogTrace(fmt.Sprintf("Expected strings: %v contain one of the strings from the DNS answer: %v , but they are not the same, raising status to warning",
				opts.ExpectedString, answerCopy))
			escalateStatus(checkers.WARNING)
		case !expectedStringsContainOneAnswerAddress:
			tryLogTrace(fmt.Sprintf("Expected strings: %v does not contain one of the strings from the DNS answer: %v , raising status to critical",
				opts.ExpectedString, answerCopy))
			escalateStatus(checkers.CRITICAL)
		}
	}

	if r.MsgHdr.Rcode != dns.RcodeSuccess {
		tryLogTrace("DNS does not have success return code, raising status to critical")
		escalateStatus(checkers.CRITICAL)
	}

	timeMetric := fmt.Sprintf(
		"time=%fs;%s;%s", queriesDuration.Seconds(),
		func() string {
			if opts.WarningTimeout != nil {
				return fmt.Sprintf("%d", *opts.WarningTimeout)
			}
			return ""
		}(),
		func() string {
			if opts.CriticalTimeout != nil {
				return fmt.Sprintf("%d", *opts.CriticalTimeout)
			}
			return ""
		}(),
	)

	msg := ""
	if len(answersWithoutHeaders) > 0 && len(answerTypes) > 0 {
		msg = fmt.Sprintf("%s returns %s (%s) |%s\n", opts.Host, answersWithoutHeaders[0], answerTypes[0], timeMetric)
	} else {
		msg = fmt.Sprintf("%s (%s) returns no answer from %s\n", opts.Host, opts.QueryType, successfulNameserver)
	}

	msg += fmt.Sprintf("HEADER-> %s\n", r.MsgHdr.String())
	for _, answer := range r.Answer {
		msg += fmt.Sprintf("ANSWER-> %s\n", answer)
	}

	return checkers.NewChecker(checkSt, msg)
}

func dnsAnswer(answer dns.RR) (string, string, error) {
	switch t := answer.(type) {
	case *dns.A:
		return t.A.String(), "A", nil
	case *dns.AAAA:
		return t.AAAA.String(), "AAAA", nil
	case *dns.MX:
		return t.Mx, "MX", nil
	case *dns.CNAME:
		return t.Target, "CNAME", nil
	default:
		return "", "", fmt.Errorf("%T is not a supported query type. Only A, AAAA, MX, CNAME are supported for expectation.", t)
	}
}

// emptyResult keeps track of why a single DNS query combination did not return any answer
// the reason is either the rcode or the query error.
type emptyResult struct {
	host       string
	nameserver string
	reason     string
}

// emptyResultsMessage builds a message from the reasons (rcodes or errors) why the DNS queries returned no answer.
// The first line contains the unique reasons from the first nameserver, each further nameserver gets a line with its own unique reasons.
func emptyResultsMessage(host string, nameservers []string, results []emptyResult) string {
	lines := make([]string, 0, len(nameservers))
	for _, nameserver := range nameservers {
		reasons := make([]string, 0)
		for _, result := range results {
			if result.nameserver == nameserver && !slices.Contains(reasons, result.reason) {
				reasons = append(reasons, result.reason)
			}
		}
		if len(reasons) == 0 {
			continue
		}
		if len(lines) == 0 {
			lines = append(lines, fmt.Sprintf("dns lookup failed for host '%s':", strings.TrimSuffix(host, ".")))
		}
		lines = append(lines, fmt.Sprintf("%s: %s", nameserver, strings.Join(reasons, ", ")))
	}
	if len(lines) == 0 {
		return "all DNS queries gave empty results or failed"
	}

	return strings.Join(lines, "\n")
}

func emptyResultReason(rcode int) string {
	rcodeStr, ok := dns.RcodeToString[rcode]
	if !ok {
		rcodeStr = fmt.Sprintf("RCODE %d", rcode)
	}
	if rcode == dns.RcodeSuccess {
		return fmt.Sprintf("no answer (%s)", rcodeStr)
	}

	return rcodeStr
}

func queryFailedReason(err error) string {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "query failed: timeout"
	}

	// unwrap to the innermost error to keep the reason short and free of nameserver addresses
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return "query failed: " + err.Error()
		}
		err = unwrapped
	}
}
