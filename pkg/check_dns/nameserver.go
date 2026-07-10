//go:build !windows

// nolint:ALL
package check_dns

import (
	"fmt"
	"net"

	"github.com/miekg/dns"
)

func adapterAddress(conf *dns.ClientConfig) (nameservers []string, err error) {
	if len(conf.Servers) == 0 {
		return nameservers, fmt.Errorf("no valid nameserver found")
	}

	parseNameserver := func(nameserver string) (string, error) {
		// ref: https://github.com/miekg/exdns/blob/d851fa434ad51cb84500b3e18b8aa7d3bead2c51/q/q.go#L148-L153
		// if the nameserver is from /etc/resolv.conf the [ and ] are already
		// added, thereby breaking net.ParseIP. Check for this and don't
		// fully qualify such a name
		if nameserver[0] == '[' && nameserver[len(nameserver)-1] == ']' {
			nameserver = nameserver[1 : len(nameserver)-1]
		}

		// ref: https://github.com/miekg/exdns/blob/d851fa434ad51cb84500b3e18b8aa7d3bead2c51/q/q.go#L154-L158
		if net.ParseIP(nameserver) == nil {
			nameserver = dns.Fqdn(nameserver)
		}
		if net.ParseIP(nameserver) == nil {
			return "", fmt.Errorf("invalid nameserver: %s", nameserver)
		}

		return nameserver, nil
	}

	parsedNameservers := make([]string, 0, len(conf.Servers))
	for _, nameserver := range conf.Servers {
		parsedNameserver, err := parseNameserver(nameserver)

		if err == nil {
			parsedNameservers = append(parsedNameservers, parsedNameserver)
		}
	}

	if len(parsedNameservers) == 0 {
		return nameservers, fmt.Errorf("could not parse any nameserver")
	}

	return parsedNameservers, nil
}

func AppendSearchPathsIfExists(host string, conf *dns.ClientConfig) string {
	if len(conf.Search) > 0 {
		return host + "." + conf.Search[0]
	}

	return host
}

func getSearchPaths(conf *dns.ClientConfig) []string {
	return conf.Search
}
