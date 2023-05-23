package snclient

import (
	"net"
	"net/netip"
	"strings"
)

type AllowedHost struct {
	Prefix       *netip.Prefix
	IP           *netip.Addr
	HostName     *string
	ResolveCache []netip.Addr
}

func NewAllowedHost(name string) AllowedHost {
	allowed := AllowedHost{}

	if strings.HasPrefix(name, "[") && strings.HasSuffix(name, "]") {
		name = strings.TrimPrefix(name, "[")
		name = strings.TrimSuffix(name, "]")
	}

	// is it a netrange?
	netRange, err := netip.ParsePrefix(name)
	if err == nil {
		allowed.Prefix = &netRange

		return allowed
	}

	// is it an ip address ipv4/ipv6
	if ip, err := netip.ParseAddr(name); err == nil {
		allowed.IP = &ip

		return allowed
	}

	allowed.HostName = &name

	return allowed
}

func (a *AllowedHost) String() string {
	switch {
	case a.Prefix != nil:
		return a.Prefix.String()
	case a.IP != nil:
		return a.IP.String()
	case a.HostName != nil:
		return *a.HostName
	}

	return ""
}

func (a *AllowedHost) Contains(addr netip.Addr, useCaching bool) bool {
	switch {
	case a.Prefix != nil:
		return a.Prefix.Contains(addr)
	case a.IP != nil:
		return a.IP.Compare(addr) == 0
	case a.HostName != nil:
		resolved := a.ResolveCache

		if useCaching || len(a.ResolveCache) == 0 {
			resolved = a.resolveCache()
			if useCaching {
				a.ResolveCache = resolved
			}
		}

		for _, i := range resolved {
			if i.Compare(addr) == 0 {
				return true
			}
		}

		return false
	}

	return false
}

func (a *AllowedHost) resolveCache() []netip.Addr {
	resolved := make([]netip.Addr, 0)

	ips, err := net.LookupIP(*a.HostName)
	if err != nil {
		log.Debugf("dns lookup for %s failed: %s", *a.HostName, err.Error())

		return resolved
	}

	for _, v := range ips {
		i, err := netip.ParseAddr(v.String())
		if err == nil {
			resolved = append(resolved, i)
		}
	}

	return resolved
}
