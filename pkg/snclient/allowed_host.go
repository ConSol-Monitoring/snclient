package snclient

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
)

type AllowedHostConfig struct {
	Allowed  []AllowedHost
	UseCache bool
}

func NewAllowedHostConfig(conf *ConfigSection) (*AllowedHostConfig, error) {
	ahc := &AllowedHostConfig{}

	// parse / set allowed hosts
	allowed, _ := conf.GetString("allowed hosts")
	if allowed != "" {
		for _, allow := range strings.Split(allowed, ",") {
			allow = strings.TrimSpace(allow)
			if allow == "" {
				continue
			}
			ahc.Allowed = append(ahc.Allowed, NewAllowedHost(allow))
		}
	}

	// parse / set cache allowed hosts
	cacheAllowedHosts, ok, err := conf.GetBool("cache allowed hosts")
	switch {
	case err != nil:
		return nil, fmt.Errorf("invalid cache allowed hosts specification: %s", err.Error())
	case ok:
		ahc.UseCache = cacheAllowedHosts
	default:
		ahc.UseCache = true
	}

	ahc.Debug()

	return ahc, nil
}

func (ahc *AllowedHostConfig) Check(remoteAddr string) bool {
	if len(ahc.Allowed) == 0 {
		return true
	}

	idx := strings.LastIndex(remoteAddr, ":")
	if idx != -1 {
		remoteAddr = remoteAddr[:idx]
	}

	if strings.HasPrefix(remoteAddr, "[") && strings.HasSuffix(remoteAddr, "]") {
		remoteAddr = strings.TrimPrefix(remoteAddr, "[")
		remoteAddr = strings.TrimSuffix(remoteAddr, "]")
	}

	addr, err := netip.ParseAddr(remoteAddr)
	if err != nil {
		log.Warnf("cannot parse remote address: %s: %s", remoteAddr, err.Error())

		return false
	}

	for _, allow := range ahc.Allowed {
		if allow.Contains(addr, ahc.UseCache) {
			return true
		}
	}

	return false
}

func (ahc *AllowedHostConfig) Debug() {
	if len(ahc.Allowed) == 0 {
		log.Debugf("allowed hosts: all")
	} else {
		log.Debugf("allowed hosts:")
		for _, allow := range ahc.Allowed {
			log.Debugf("    - %s", allow.String())
		}
	}
}

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
