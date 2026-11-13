//go:build windows

package snclient

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestICMPReplyLayout(t *testing.T) {
	var reply4 icmpEchoReply
	var reply6 icmpv6EchoReply

	if unsafe.Sizeof(uintptr(0)) == 8 {
		assert.Equal(t, uintptr(40), unsafe.Sizeof(reply4))
		assert.Equal(t, uintptr(24), unsafe.Offsetof(reply4.Options))
		assert.Equal(t, uintptr(0), unsafe.Offsetof(reply4.Options.TTL))
		assert.Equal(t, uintptr(8), unsafe.Offsetof(reply4.Options.OptionsData))

		assert.Equal(t, uintptr(36), unsafe.Sizeof(reply6))
		assert.Equal(t, uintptr(28), unsafe.Offsetof(reply6.Status))
		assert.Equal(t, uintptr(32), unsafe.Offsetof(reply6.RoundTripTime))

		return
	}

	assert.Equal(t, uintptr(28), unsafe.Sizeof(reply4))
	assert.Equal(t, uintptr(20), unsafe.Offsetof(reply4.Options))
	assert.Equal(t, uintptr(0), unsafe.Offsetof(reply4.Options.TTL))
	assert.Equal(t, uintptr(4), unsafe.Offsetof(reply4.Options.OptionsData))

	assert.Equal(t, uintptr(36), unsafe.Sizeof(reply6))
	assert.Equal(t, uintptr(28), unsafe.Offsetof(reply6.Status))
	assert.Equal(t, uintptr(32), unsafe.Offsetof(reply6.RoundTripTime))
}

func TestPingEntry(t *testing.T) {
	chk := &CheckPing{packets: 4}

	entry, err := chk.pingEntry([]pingReply{
		{rtt: 2, ttl: 128, status: ipSuccess},
		{rtt: 4, ttl: 64, status: ipSuccess},
		{status: ipReqTimedOut},
		{err: errors.New("icmp failed")},
	})
	require.NoError(t, err)
	assert.Equal(t, "4", entry["sent"])
	assert.Equal(t, "2", entry["received"])
	assert.Equal(t, "50.000", entry["pl"])
	assert.Equal(t, "3.000", entry["rta"])
	assert.Equal(t, "128", entry["ttl"])

	entry, err = chk.pingEntry([]pingReply{
		{status: ipReqTimedOut},
		{status: ipReqTimedOut},
		{status: ipReqTimedOut},
		{status: ipReqTimedOut},
	})
	require.NoError(t, err)
	assert.Equal(t, "100.000", entry["pl"])
	assert.Empty(t, entry["rta"])
	assert.Empty(t, entry["ttl"])

	_, err = chk.pingEntry([]pingReply{{err: errors.New("icmp failed")}})
	assert.Error(t, err)
}

func TestCollectPingReplies(t *testing.T) {
	replies := make(chan indexedPingReply, 2)
	replies <- indexedPingReply{index: 1, reply: pingReply{status: ipSuccess, rtt: 5}}
	replies <- indexedPingReply{index: 0, reply: pingReply{status: ipReqTimedOut}}

	results := collectPingReplies(replies, 2, 3, context.Canceled)
	require.Len(t, results, 3)
	assert.Equal(t, uint32(ipReqTimedOut), results[0].status)
	assert.Equal(t, uint32(ipSuccess), results[1].status)
	assert.Equal(t, uint32(5), results[1].rtt)
	assert.ErrorIs(t, results[2].err, context.Canceled)
}

func TestICMPStatusFromError(t *testing.T) {
	status, ok := icmpStatusFromError(syscall.Errno(ipReqTimedOut))
	assert.True(t, ok)
	assert.Equal(t, uint32(ipReqTimedOut), status)

	status, ok = icmpStatusFromError(fmt.Errorf("wrapped: %w", syscall.Errno(11003)))
	assert.True(t, ok)
	assert.Equal(t, uint32(11003), status)

	_, ok = icmpStatusFromError(syscall.Errno(5))
	assert.False(t, ok)
}

func TestICMPCreateFile(t *testing.T) {
	require.NoError(t, loadICMP())

	handle, _, callErr := procIcmpCreateFile.Call()
	if !assert.False(t, handle == 0 || windows.Handle(handle) == windows.InvalidHandle,
		"IcmpCreateFile failed: %v", callErr) {
		return
	}
	procIcmpCloseHandle.Call(handle) //nolint:errcheck // test cleanup
}
