// SPDX-License-Identifier: MIT
//
// The status sample in this file is reused from the windows_exporter project,
// which is MIT licensed (Copyright (c) 2016 Martin Lindhe,
// Copyright (c) 2021 The Prometheus Authors).

// The status sample in this file was recorded by the windows_exporter project
// and is reused here to test the ported parser.

//go:build windows

package snclient

import (
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// statusSample is the W32TIME_STATUS_INFO that W32TimeQueryStatus returned
// on a Windows Server 2025 Hyper-V guest, up to the source string. It was
// recorded by the windows_exporter project (MIT, Copyright The Prometheus
// Authors) and is reused to test the ported parser.
const statusSample = "" +
	"78000000 00000000 01000000 06000000" +
	"564d5450 00000000 f0b7a4eb a867dd01" +
	"e5060000 00000000 a2860100 00000000" +
	"e9ffffff 00000000 e8e5a530 70010000" +
	"c0fdffff ffffffff 02000000 03000000" +
	"59620200 40000000 00000000 00000000" +
	"f6ccb901 00000000 00000000 00000000" +
	"00000000 00000000"

func TestParseW32TimeStatus(t *testing.T) {
	raw, err := hex.DecodeString(strings.ReplaceAll(statusSample, " ", ""))
	require.NoError(t, err)
	require.Len(t, raw, int(unsafe.Sizeof(w32timeStatusInfo{})))

	source, err := windows.UTF16PtrFromString("VM IC Time Synchronization Provider")
	require.NoError(t, err)

	// The sample points into the native buffer, point it at a Go string instead.
	binary.LittleEndian.PutUint64(raw[unsafe.Offsetof(w32timeStatusInfo{}.wszSource):], uint64(uintptr(unsafe.Pointer(source))))

	info := new(w32timeStatusInfo)
	copy(unsafe.Slice((*byte)(unsafe.Pointer(info)), len(raw)), raw)

	status, err := parseW32TimeStatus(info)
	require.NoError(t, err)
	require.True(t, status.LastSuccessfulSync.Equal(time.Date(2026, 10, 29, 13, 25, 9, 659032000, time.UTC)),
		status.LastSuccessfulSync.String())

	status.LastSuccessfulSync = time.Time{}
	require.Equal(t, w32timeStatus{
		LeapIndicator:  0,
		Stratum:        1,
		RootDelay:      176500 * time.Nanosecond,
		RootDispersion: 10000200 * time.Nanosecond,
		Source:         "VM IC Time Synchronization Provider",
		PhaseOffset:    -57600 * time.Nanosecond,
		State:          w32timeStateSync,
		LastSyncResult: w32timeSyncResultSuccess,
	}, status)
}

func TestParseW32TimeStatusRejectsShortStructure(t *testing.T) {
	_, err := parseW32TimeStatus(&w32timeStatusInfo{ulSize: 64})
	require.ErrorContains(t, err, "unexpected W32TIME_STATUS_INFO size 64")
}

func TestCheckNTPOffsetW32Time(t *testing.T) {
	snc := StartTestAgent(t, "")
	defer StopTestAgent(t, snc)

	res := snc.RunCheck("check_ntp_offset", []string{"source=w32time"})
	output := string(res.BuildPluginOutput())

	if strings.Contains(output, "the Windows Time service is not running") {
		t.Skip("windows time service is not running")
	}

	// a service in hold, spike or unset state reports no metrics
	if strings.Contains(output, "State Machine:") {
		t.Skip("windows time service is not synchronized")
	}

	assert.Contains(t, output, "'offset'=")
	assert.Contains(t, output, "'stratum'=")
	assert.NotContains(t, output, "'jitter'=")
}
