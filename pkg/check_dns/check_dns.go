// nolint:ALL
package check_dns

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

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

	ckr := opts.run()
	fmt.Fprintf(output, "%s - %s", ckr.Status, strings.TrimSpace(ckr.Message))

	return int(ckr.Status)
}

// adopted from https://raw.githubusercontent.com/mackerelio/go-check-plugins/master/check-dns/lib/
// Apache-2.0 license
type dnsOpts struct {
	Host           string   `short:"H" long:"host" required:"true" description:"The name or address you want to query"`
	Server         string   `short:"s" long:"server" description:"DNS server you want to use for the lookup"`
	Port           int      `short:"p" long:"port" default:"53" description:"Port number you want to use"`
	QueryType      string   `short:"q" long:"querytype" default:"A" description:"DNS record query type"`
	Norec          bool     `long:"norec" description:"Set not recursive mode"`
	ExpectedString []string `short:"e" long:"expected-string" description:"IP-ADDRESS string you expect the DNS server to return. If multiple IP-ADDRESS are returned at once, you have to specify whole string"`
}

func parseArgs(args []string) (*dnsOpts, error) {
	opts := &dnsOpts{}
	psr := flags.NewParser(opts, flags.HelpFlag|flags.PassDoubleDash) // default flags without flags.PrintErrors
	psr.Name = "check_dns"
	_, err := psr.ParseArgs(args)
	return opts, err
}

func (opts *dnsOpts) run() *checkers.Checker {
	var nameserver string
	var err error
	if opts.Server != "" {
		nameserver = opts.Server
	} else {
		nameserver, err = adapterAddress()
		if err != nil {
			return checkers.Critical(err.Error())
		}
	}
	nameserver = net.JoinHostPort(nameserver, strconv.Itoa(opts.Port))

	queryType, ok := dns.StringToType[strings.ToUpper(opts.QueryType)]
	if !ok {
		return checkers.Critical(fmt.Sprintf("%s is invalid query type", opts.QueryType))
	}

	c := new(dns.Client)
	m := &dns.Msg{
		MsgHdr: dns.MsgHdr{
			RecursionDesired: !opts.Norec,
			Opcode:           dns.OpcodeQuery,
		},
		Question: []dns.Question{{Name: dns.Fqdn(opts.Host), Qtype: queryType, Qclass: dns.StringToClass["IN"]}},
	}
	m.Id = dns.Id()

	r, _, err := c.Exchange(m, nameserver)
	if err != nil {
		return checkers.Critical(err.Error())
	}

	checkSt := checkers.OK
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
		match := 0
		for _, expectedString := range opts.ExpectedString {
			for _, answer := range r.Answer {
				anserWithoutHeader, _, err := dnsAnswer(answer)
				if err != nil {
					return checkers.Critical(err.Error())
				}
				if anserWithoutHeader == expectedString {
					match += 1
				}
			}
		}
		if match == len(r.Answer) {
			if len(opts.ExpectedString) == len(r.Answer) { // case 1
				checkSt = checkers.OK
			} else { // case 2
				checkSt = checkers.WARNING
			}
		} else {
			if match > 0 { // case 3,4
				checkSt = checkers.WARNING
			} else { // case 5,6
				checkSt = checkers.CRITICAL
			}
		}
	}

	if r.MsgHdr.Rcode != dns.RcodeSuccess {
		checkSt = checkers.CRITICAL
	}

	msg := ""
	if len(r.Answer) > 0 {
		res, dnsType, err := dnsAnswer(r.Answer[0])
		if err != nil {
			msg = err.Error()
		} else {
			msg = fmt.Sprintf("%s returns %s (%s)\n", opts.Host, res, dnsType)
		}
	} else {
		msg = fmt.Sprintf("%s (%s) returns no answer from %s\n", opts.Host, opts.QueryType, nameserver)
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
