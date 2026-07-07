//go:build windows

// nolint:ALL
package check_dns

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"unsafe"

	"github.com/miekg/dns"
	"golang.org/x/sys/windows"
)

// ref: https://go.dev/src/net/interface_windows.go
// windows does not use a resolv.config file i.e dns.ClientConfig
// this function is defined for consistency across platforms
func adapterAddress(_ *dns.ClientConfig) (nameservers []string, err error) {
	var b []byte
	l := uint32(15000) // recommended initial size
	for {
		b = make([]byte, l)
		err := windows.GetAdaptersAddresses(syscall.AF_UNSPEC, windows.GAA_FLAG_INCLUDE_PREFIX, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&b[0])), &l)
		if err == nil {
			if l == 0 {
				return nameservers, nil
			}
			break
		}
		if err.(syscall.Errno) != syscall.ERROR_BUFFER_OVERFLOW {
			return nameservers, os.NewSyscallError("syscall failed: GetAdaptersAddresses", err)
		}
		if l <= uint32(len(b)) {
			return nameservers, os.NewSyscallError("syscall failed: GetAdaptersAddresses", err)
		}
	}
	var aas []*windows.IpAdapterAddresses
	for aa := (*windows.IpAdapterAddresses)(unsafe.Pointer(&b[0])); aa != nil; aa = aa.Next {
		aas = append(aas, aa)
	}
	if len(aas) == 0 {
		return nameservers, fmt.Errorf("no valid nameserver found")
	}
	nameserver := aas[0].FirstDnsServerAddress.Address.IP().String()
	// ref: https://github.com/miekg/exdns/blob/d851fa434ad51cb84500b3e18b8aa7d3bead2c51/q/q.go#L154-L158
	if net.ParseIP(nameserver) == nil {
		nameserver = dns.Fqdn(nameserver)
	}
	if net.ParseIP(nameserver) == nil {
		return nameservers, fmt.Errorf("invalid nameserver: %s", nameserver)
	}
	return []string{nameserver}, nil
}

// windows does not use a resolv.config file i.e dns.ClientConfig
// this function is defined for consistency across platforms
func AppendSearchPathsIfExists(host string, _ *dns.ClientConfig) string {
	return host
}

func getSearchPaths(_ *dns.ClientConfig) []string {
	return []string{}
}
