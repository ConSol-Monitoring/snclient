//go:build windows

package snclient

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	icmpDefaultTimeout = 4 * time.Second // ping.exe default
	icmpPacketSize     = 32              // ping.exe default payload
	icmpSendInterval   = time.Second
	icmpReplySlack     = 24  // ICMP error message and IO_STATUS_BLOCK
	icmpPacketLossMax  = 100 // complete packet loss in percent

	ipSuccess        = 0
	ipStatusMin      = 11001 // IP_BUF_TOO_SMALL
	ipStatusMax      = 11045 // IP_DEST_SCOPE_MISMATCH, the last IPv6 status
	ipReqTimedOut    = 11010
	ipGeneralFailure = 11050
)

var (
	iphlpapiDLL = windows.NewLazySystemDLL("iphlpapi.dll")

	procIcmpCreateFile  = iphlpapiDLL.NewProc("IcmpCreateFile")
	procIcmpCloseHandle = iphlpapiDLL.NewProc("IcmpCloseHandle")
	procIcmpSendEcho    = iphlpapiDLL.NewProc("IcmpSendEcho")
	procIcmp6CreateFile = iphlpapiDLL.NewProc("Icmp6CreateFile")
	procIcmp6SendEcho2  = iphlpapiDLL.NewProc("Icmp6SendEcho2")

	loadICMPOnce sync.Once
	errLoadICMP  error
)

// ipOptionInformation mirrors IP_OPTION_INFORMATION (ipexport.h).
type ipOptionInformation struct {
	TTL         uint8
	TOS         uint8
	Flags       uint8
	OptionsSize uint8
	OptionsData uintptr
}

// icmpEchoReply mirrors ICMP_ECHO_REPLY (ipexport.h).
// IcmpSendEcho parses the reply buffer itself, so the native layout is used on 64 bit as well.
type icmpEchoReply struct {
	Address       uint32
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	Data          uintptr
	Options       ipOptionInformation
}

// icmpv6EchoReply mirrors ICMPV6_ECHO_REPLY (ipexport.h).
// Its IPV6_ADDRESS_EX prefix is packed and 26 bytes long, so Status starts at offset 28.
// Unlike IPv4, the API returns no TTL for IPv6.
type icmpv6EchoReply struct {
	Address       [26]byte
	_             [2]byte
	Status        uint32
	RoundTripTime uint32
}

type pingReply struct {
	rtt    uint32
	ttl    uint8
	status uint32
	err    error
}

type indexedPingReply struct {
	index int
	reply pingReply
}

// loadICMP checks that iphlpapi.dll exports every function this file calls.
func loadICMP() error {
	loadICMPOnce.Do(func() {
		for _, proc := range []*windows.LazyProc{
			procIcmpCreateFile, procIcmpCloseHandle, procIcmpSendEcho,
			procIcmp6CreateFile, procIcmp6SendEcho2,
		} {
			if err := proc.Find(); err != nil {
				errLoadICMP = fmt.Errorf("iphlpapi.dll: %w", err)

				return
			}
		}
	})

	return errLoadICMP
}

func (l *CheckPing) addPingWindows(ctx context.Context, check *CheckData) error {
	if l.ipv4 && l.ipv6 {
		return fmt.Errorf("cannot use -4 and -6 together")
	}
	if l.packets < 1 {
		return fmt.Errorf("packets must be greater than 0")
	}
	if err := loadICMP(); err != nil {
		return err
	}

	ips, err := l.resolvePingHost(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("ping: %w", ctx.Err())
		}

		return l.addPingResolveError(check)
	}

	perPacket := icmpDefaultTimeout
	if l.timeout > 0 {
		perPacket = time.Duration(l.timeout) * time.Second
	}

	// Leave one second for the check to finalize, like the command variant.
	pingCtx, cancel := context.WithDeadline(ctx, time.Now().Add(check.timeout-time.Second))
	defer cancel()

	results := l.pingAddress(pingCtx, ips[0], perPacket)
	entry, err := l.pingEntry(results)
	if err != nil {
		return err
	}

	check.listData = append(check.listData, entry)
	l.addMetrics(check, entry)

	return nil
}

// resolvePingHost returns the addresses in resolver order.
// This matches the address family ping.exe would use.
func (l *CheckPing) resolvePingHost(ctx context.Context) ([]net.IP, error) {
	network := "ip"
	switch {
	case l.ipv4:
		network = "ip4"
	case l.ipv6:
		network = "ip6"
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, network, l.hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve hostname: %w", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("failed to resolve hostname")
	}

	return ips, nil
}

func (l *CheckPing) addPingResolveError(check *CheckData) error {
	entry := l.defaultEntry()
	entry["_error"] = "failed to resolve hostname"
	entry["pl"] = fmt.Sprintf("%.3f", float64(icmpPacketLossMax))
	check.listData = append(check.listData, entry)
	l.addMetrics(check, entry)

	return nil
}

// pingAddress sends one echo per second, like ping.exe.
// Every packet runs in its own goroutine because IcmpSendEcho is synchronous.
func (l *CheckPing) pingAddress(ctx context.Context, address net.IP, perPacket time.Duration) []pingReply {
	packets := int(l.packets)
	replies := make(chan indexedPingReply, packets)
	sent := 0

	ticker := time.NewTicker(icmpSendInterval)
	defer ticker.Stop()

	for sent < packets {
		if sent > 0 {
			select {
			case <-ticker.C:
			case <-ctx.Done(): // ctx has a timeout based on check.timeout
				return collectPingReplies(replies, sent, packets, ctx.Err())
			}
		}

		index := sent
		go func() {
			replies <- indexedPingReply{
				index: index,
				reply: l.pingOnce(ctx, address, perPacket),
			}
		}()
		sent++
	}

	return collectPingReplies(replies, sent, packets, nil)
}

func collectPingReplies(replies <-chan indexedPingReply, sent, packets int, err error) []pingReply {
	results := make([]pingReply, packets)
	for i := range results {
		results[i] = pingReply{status: ipReqTimedOut}
	}
	if err != nil {
		for i := sent; i < packets; i++ {
			results[i] = pingReply{err: err}
		}
	}

	for range sent {
		result := <-replies
		results[result.index] = result.reply
	}

	return results
}

func (l *CheckPing) pingOnce(ctx context.Context, address net.IP, perPacket time.Duration) pingReply {
	select {
	case <-ctx.Done():
		return pingReply{err: ctx.Err()}
	default:
	}

	timeout := perPacket
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < timeout {
			timeout = remaining
		}
	}
	if timeout <= 0 {
		return pingReply{status: ipReqTimedOut}
	}

	if address.To4() != nil {
		rtt, ttl, status, err := icmpEcho4(address, timeout)

		return pingReply{rtt: rtt, ttl: ttl, status: status, err: err}
	}

	rtt, ttl, status, err := icmpEcho6(address, timeout)

	return pingReply{rtt: rtt, ttl: ttl, status: status, err: err}
}

func (l *CheckPing) pingEntry(results []pingReply) (map[string]string, error) {
	entry := l.defaultEntry()
	sent := int(l.packets)
	received := 0
	var rttSum uint64
	var ttl uint8
	var firstErr error

	for _, result := range results {
		switch {
		case result.err != nil:
			if firstErr == nil {
				firstErr = result.err
			}
		case result.status == ipSuccess:
			received++
			rttSum += uint64(result.rtt)
			if ttl == 0 {
				ttl = result.ttl
			}
		}
	}

	if received == 0 && firstErr != nil {
		return nil, fmt.Errorf("ping failed: %w", firstErr)
	}
	if firstErr != nil {
		log.Debugf("ping: %s", firstErr)
	}

	entry["sent"] = strconv.Itoa(sent)
	entry["received"] = strconv.Itoa(received)
	entry["pl"] = fmt.Sprintf("%.3f", float64(sent-received)*100/float64(sent))
	if received > 0 {
		rta := float64(rttSum) / float64(received)
		entry["rta"] = fmt.Sprintf("%.3f", rta)
		if ttl > 0 {
			entry["ttl"] = strconv.Itoa(int(ttl))
		}
	}

	return entry, nil
}

func icmpEcho4(address net.IP, timeout time.Duration) (rtt uint32, ttl uint8, status uint32, err error) {
	handle, _, callErr := procIcmpCreateFile.Call()
	if handle == 0 || windows.Handle(handle) == windows.InvalidHandle {
		return 0, 0, 0, fmt.Errorf("IcmpCreateFile: %w", callErr)
	}
	defer procIcmpCloseHandle.Call(handle) //nolint:errcheck // a close failure cannot be handled

	ip4 := address.To4()
	if ip4 == nil {
		return 0, 0, 0, fmt.Errorf("not an IPv4 address: %s", address)
	}

	payload := make([]byte, icmpPacketSize)
	buf := make([]byte, int(unsafe.Sizeof(icmpEchoReply{}))+len(payload)+icmpReplySlack)

	count, _, callErr := procIcmpSendEcho.Call(
		handle,
		uintptr(binary.LittleEndian.Uint32(ip4)),
		uintptr(unsafe.Pointer(&payload[0])),
		uintptr(len(payload)),
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		icmpTimeoutMillis(timeout),
	)
	if count == 0 {
		if ipStatus, ok := icmpStatusFromError(callErr); ok {
			return 0, 0, ipStatus, nil
		}

		return 0, 0, 0, fmt.Errorf("IcmpSendEcho: %w", callErr)
	}

	reply := (*icmpEchoReply)(unsafe.Pointer(&buf[0]))

	return reply.RoundTripTime, reply.Options.TTL, reply.Status, nil
}

func icmpEcho6(address net.IP, timeout time.Duration) (rtt uint32, ttl uint8, status uint32, err error) {
	handle, _, callErr := procIcmp6CreateFile.Call()
	if handle == 0 || windows.Handle(handle) == windows.InvalidHandle {
		return 0, 0, 0, fmt.Errorf("Icmp6CreateFile: %w", callErr)
	}
	defer procIcmpCloseHandle.Call(handle) //nolint:errcheck // a close failure cannot be handled

	ip16 := address.To16()
	if ip16 == nil {
		return 0, 0, 0, fmt.Errorf("not an IPv6 address: %s", address)
	}

	var source, destination windows.RawSockaddrInet6
	source.Family = windows.AF_INET6
	destination.Family = windows.AF_INET6
	copy(destination.Addr[:], ip16)

	payload := make([]byte, icmpPacketSize)
	buf := make([]byte, int(unsafe.Sizeof(icmpv6EchoReply{}))+len(payload)+icmpReplySlack)

	count, _, callErr := procIcmp6SendEcho2.Call(
		handle,
		0, // Event
		0, // ApcRoutine
		0, // ApcContext
		uintptr(unsafe.Pointer(&source)),
		uintptr(unsafe.Pointer(&destination)),
		uintptr(unsafe.Pointer(&payload[0])),
		uintptr(len(payload)),
		0, // RequestOptions
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		icmpTimeoutMillis(timeout),
	)
	if count == 0 {
		if ipStatus, ok := icmpStatusFromError(callErr); ok {
			return 0, 0, ipStatus, nil
		}

		return 0, 0, 0, fmt.Errorf("Icmp6SendEcho2: %w", callErr)
	}

	reply := (*icmpv6EchoReply)(unsafe.Pointer(&buf[0]))

	return reply.RoundTripTime, 0, reply.Status, nil
}

// icmpStatusFromError returns the IP_STATUS in a failed echo call.
// A timeout and an unreachable destination are packet loss, not a broken check.
func icmpStatusFromError(err error) (uint32, bool) {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return 0, false
	}
	if errno == 0 {
		return ipGeneralFailure, true
	}

	status := uint32(errno)
	switch {
	case status >= ipStatusMin && status <= ipStatusMax:
		return status, true
	case status == ipGeneralFailure:
		return status, true
	default:
		return 0, false
	}
}

func icmpTimeoutMillis(timeout time.Duration) uintptr {
	millis := max(timeout.Milliseconds(), 1)

	return uintptr(millis)
}
