// nolint:ALL
package check_dns

import (
	"context"
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
		fmt.Fprintf(output, "%s", err.Error())
		return 2
	}

	ckr := opts.run(ctx)
	fmt.Fprintf(output, "%s - %s", ckr.Status, strings.TrimSpace(ckr.Message))

	return int(ckr.Status)
}

// adopted from https://raw.githubusercontent.com/mackerelio/go-check-plugins/master/check-dns/lib/
// Apache-2.0 license
type dnsOpts struct {
	Host            string   `short:"H" long:"host" required:"true" description:"The name or address you want to query"`
	Server          string   `short:"s" long:"server" description:"DNS server you want to use for the lookup"`
	Port            int      `short:"p" long:"port" default:"53" description:"Port number you want to use"`
	QueryType       string   `short:"q" long:"querytype" default:"A" description:"DNS record query type"`
	Norec           bool     `long:"norec" description:"Set not recursive mode"`
	ExpectedString  []string `short:"e" long:"expected-string" description:"IP-ADDRESS string you expect the DNS server to return. If multiple IP-ADDRESS are returned at once, you have to specify whole string"`
	SearchPaths     []string `long:"search-path" description:"Search paths is added to the domains before sending a DNS query. This can be specified multiple times."`
	ResolvConfFile  string   `long:"resolv-conf-file" default:"/etc/resolv.conf" description:"Path to the resolv.conf file to use. Is not used in Windows. Default is /etc/resolv.conf ."`
	Verbose         bool     `short:"v" long:"vv" long:"vvv" long:"verbose" description:"Show verbose output."`
	WarningTimeout  *int     `short:"w" long:"warning" description:"Warning timeout, if getting a successfull DNS query takes longer than specified, set return status to warning."`
	CriticalTimeout *int     `short:"c" long:"critical" description:"Critical timeout, if getting a successfull DNS query takes longer than specified, set return status to critical."`
	Timeout         int      `short:"t" long:"timeout" default:"10" description:"If the program cannot get a successfull DNS response until the specified timeout, it exit with critical status."`
}

func parseArgs(args []string) (*dnsOpts, error) {
	opts := &dnsOpts{}
	psr := flags.NewParser(opts, flags.HelpFlag|flags.PassDoubleDash) // default flags without flags.PrintErrors
	psr.Name = "check_dns"
	_, err := psr.ParseArgs(args)
	return opts, err
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
	if opts.Server != "" {
		nameservers = []string{opts.Server}
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
		return checkers.Critical(fmt.Sprintf("%s is invalid query type", opts.QueryType))
	}

	c := new(dns.Client)

	var lastErr error
	var r *dns.Msg
	var duration time.Duration

	var successfulNameserver string
	var successfulDuration time.Duration
	var successfulHost string

	queryDNSChan := make(chan bool, 1)
	queryDNSSuccessfull := false

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
						tryLogTrace(fmt.Sprintf("DNS query returned empty result, continuing to next combination, host: %s, nameserver: %s, duration: %dms",
							hostCandidate, nameserver, duration.Milliseconds()))

						continue
					}

					successfulNameserver = nameserver
					successfulHost = hostCandidate
					successfulDuration = duration
					gotAnswer = true
					tryLogDebug(fmt.Sprintf("successfully queried DNS, host: %s, nameserver: %s, duration: %dms", successfulHost, successfulNameserver, successfulDuration.Milliseconds()))

					break
				}

				tryLogTrace(fmt.Sprintf("DNS query failed, host: %s, nameserver: %s, duration: %dms", hostCandidate, nameserver, duration.Milliseconds()))

				lastErr = err
			}

			if gotAnswer {
				break
			}
		}

		queryDNSChan <- gotAnswer
	}

	queryBeginTimestamp := time.Now()
	go queryDNS()

	select {
	case <-time.After(time.Duration(opts.Timeout) * time.Second):
		return checkers.Critical(fmt.Sprintf("Did not get a successfull DNS query in timeout: %d seconds", opts.Timeout))
	case queryDNSSuccessfull = <-queryDNSChan:
		break
	}
	queryEndTimestamp := time.Now()
	queryDuration := queryEndTimestamp.Sub(queryBeginTimestamp)

	if !queryDNSSuccessfull {
		return checkers.Critical(fmt.Sprintf("all attempts failed, last error: %v", lastErr))
	}

	if r == nil {
		return checkers.Critical(fmt.Sprintf("all attempts failed, last error: %v", lastErr))
	}

	checkSt := checkers.OK

	escalateStatus := func(newStatus checkers.Status) {
		if newStatus > checkSt {
			checkSt = newStatus
			tryLogTrace(fmt.Sprintf("status escalated to %s", checkSt.String()))
		}
	}

	switch {
	case opts.CriticalTimeout != nil && queryDuration.Seconds() > float64(*opts.CriticalTimeout):
		tryLogTrace(fmt.Sprintf("DNS query took %f seconds, which is higher than the critical threshold: %d", queryDuration.Seconds(), opts.CriticalTimeout))
		escalateStatus(checkers.CRITICAL)
	case opts.WarningTimeout != nil && queryDuration.Seconds() > float64(*opts.WarningTimeout):
		tryLogTrace(fmt.Sprintf("DNS query took %f seconds, which is higher than the warning threshold: %d", queryDuration.Seconds(), opts.WarningTimeout))
		escalateStatus(checkers.WARNING)
	default:
		tryLogTrace(fmt.Sprintf("DNS query took %f seconds, and it is lower than (if specified) warning threshold: %v and critical threshold: %v",
			queryDuration.Seconds(), opts.WarningTimeout, opts.CriticalTimeout))
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
			return checkers.Critical(fmt.Sprintf("%s is not supported query type. Only A, AAAA, MX, CNAME are supported query types.", opts.QueryType))
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
		default:
			tryLogTrace(fmt.Sprintf("Could not comapre expected strings: %v with the strings from the DNS answer: %v , raising status to unknown",
				opts.ExpectedString, answerCopy))
			escalateStatus(checkers.UNKNOWN)
		}
	}

	if r.MsgHdr.Rcode != dns.RcodeSuccess {
		tryLogTrace("DNS does not have success return code, raising status to critical")
		checkSt = checkers.CRITICAL
		escalateStatus(checkers.UNKNOWN)
	}

	msg := ""
	if len(answersWithoutHeaders) > 0 && len(answerTypes) > 0 {
		msg = fmt.Sprintf("%s returns %s (%s)\n", opts.Host, answersWithoutHeaders[0], answerTypes[0])
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
		return "", "", fmt.Errorf("%T is not supported query type. Only A, AAAA, MX, CNAME is supported for expectation.", t)
	}
}
