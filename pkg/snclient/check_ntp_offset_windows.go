// SPDX-License-Identifier: MIT
//
// Ported from the windows_exporter project, which is MIT licensed
// (Copyright (c) 2016 Martin Lindhe, Copyright (c) 2021 The Prometheus Authors).
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// The windows part of the w32time source of check_ntp_offset is a port of the
// W32Time support of the windows_exporter,
// https://github.com/prometheus-community/windows_exporter, which was added in
// prometheus-community/windows_exporter#2813 ("time: add W32Time sync status
// metrics"):
//
//   - internal/headers/w32time/w32time.go
//   - the sync_status collector of internal/collector/time/time.go
//
// The W32TIME_STATUS_INFO layout, the field interpretation and the
// W32TimeQueryStatus/W32TimeBufferFree sequence are taken from there. The
// layout follows the W32Time Remote Protocol [MS-W32T] and was verified
// against w32tm /query /status on windows:
//
// [MS-W32T]: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-w32t/0e425c15-8ae4-4c2a-b431-84a66b92986a
//
// It is not a plain copy. For snclient the lazyproc helper of the
// windows_exporter was replaced by a local sync.Once, the status types are
// shared with the platform independent check code, the poll interval and
// precision fields are skipped and the status is evaluated by the check
// instead of being exported as prometheus metrics.

//go:build windows

package snclient

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	rpcServerUnavailable = 1722 // RPC_S_SERVER_UNAVAILABLE, W32Time is not running.
	lastSyncTicksOffset  = 24   // offset of qwLastSyncTicks, 8 byte aligned on 64 bit windows only.
)

var (
	w32timeDLL = windows.NewLazySystemDLL("w32time.dll")

	procW32TimeQueryStatus = w32timeDLL.NewProc("W32TimeQueryStatus")
	procW32TimeBufferFree  = w32timeDLL.NewProc("W32TimeBufferFree")

	loadW32TimeOnce sync.Once
	errW32TimeLoad  error
)

// loadW32Time checks that w32time.dll exports every function this file calls,
// a missing export would panic on the first call otherwise.
func loadW32Time() error {
	loadW32TimeOnce.Do(func() {
		for _, proc := range []*windows.LazyProc{procW32TimeQueryStatus, procW32TimeBufferFree} {
			if err := proc.Find(); err != nil {
				errW32TimeLoad = fmt.Errorf("w32time.dll: %w", err)

				return
			}
		}
	})

	return errW32TimeLoad
}

// w32timeStatusInfo mirrors W32TIME_STATUS_INFO ([MS-W32T] 2.2.17) as the
// 64 bit C ABI lays it out in memory, which only windows/amd64 and windows/arm64
// reproduce: on 386 Go aligns the 64 bit fields to 4 bytes instead of 8, so
// every field after qwLastSyncTicks moves 4 bytes up. parseW32TimeStatus checks
// that offset and rejects the structure instead of reading wrong values.
type w32timeStatusInfo struct {
	ulSize                uint32
	eLeapIndicator        uint32
	nStratum              uint32
	nPollInterval         int32 // log2 of the poll interval in seconds.
	refidSource           uint32
	qwLastSyncTicks       uint64 // FILETIME of the last successful synchronization.
	toRootDelay           int64  // 100 ns units.
	tpRootDispersion      uint64 // 100 ns units.
	nClockPrecision       int32  // log2 of the clock precision in seconds.
	wszSource             *uint16
	toSysPhaseOffset      int64 // 100 ns units.
	ulLcState             uint32
	ulTSFlags             uint32
	ulClockRate           uint32
	ulNetlogonServiceBits uint32
	eLastSyncResult       uint32
	tpTimeLastGoodSync    uint64 // Time since the last good synchronization, 100 ns units.
	cEntries              uint32
	pEntries              uintptr
}

// queryW32TimeStatus returns the status of the local Windows Time service. It
// needs no administrative rights, but fails when the service is not running.
func queryW32TimeStatus() (w32timeStatus, error) {
	if err := loadW32Time(); err != nil {
		return w32timeStatus{}, err
	}

	var info *w32timeStatusInfo

	result, _, _ := syscall.SyscallN(procW32TimeQueryStatus.Addr(), 0, uintptr(unsafe.Pointer(&info)))
	if result != 0 {
		if result == rpcServerUnavailable {
			return w32timeStatus{}, fmt.Errorf("w32time.dll: the Windows Time service is not running: %w", windows.Errno(result))
		}

		return w32timeStatus{}, fmt.Errorf("W32TimeQueryStatus: %w", windows.Errno(result))
	}

	if info == nil {
		return w32timeStatus{}, fmt.Errorf("W32TimeQueryStatus: %w", windows.ERROR_INVALID_DATA)
	}

	// The strings are part of the same buffer, so only the outer buffer is freed.
	//nolint:errcheck // the dll owns the buffer, a failure here cannot be handled
	defer syscall.SyscallN(procW32TimeBufferFree.Addr(), uintptr(unsafe.Pointer(info)))

	return parseW32TimeStatus(info)
}

//nolint:gosec // G115: the 100ns units and the FILETIME halves come from the service
func parseW32TimeStatus(info *w32timeStatusInfo) (w32timeStatus, error) {
	// On 386 Go shifts the 64 bit fields, which the size check below cannot catch.
	if unsafe.Offsetof(info.qwLastSyncTicks) != lastSyncTicksOffset {
		return w32timeStatus{}, fmt.Errorf("W32TimeQueryStatus: unsupported W32TIME_STATUS_INFO layout on this architecture: %w", errors.ErrUnsupported)
	}

	if uintptr(info.ulSize) < unsafe.Sizeof(*info) {
		return w32timeStatus{}, fmt.Errorf("W32TimeQueryStatus: unexpected W32TIME_STATUS_INFO size %d, want %d",
			info.ulSize, unsafe.Sizeof(*info))
	}

	status := w32timeStatus{
		LeapIndicator:  info.eLeapIndicator,
		Stratum:        info.nStratum,
		RootDelay:      time.Duration(info.toRootDelay) * 100,
		RootDispersion: time.Duration(info.tpRootDispersion) * 100,
		PhaseOffset:    time.Duration(info.toSysPhaseOffset) * 100,
		State:          w32timeState(info.ulLcState),
		LastSyncResult: w32timeSyncResult(info.eLastSyncResult),
	}

	if info.wszSource != nil {
		status.Source = windows.UTF16PtrToString(info.wszSource)
	}

	if info.qwLastSyncTicks != 0 {
		lastSync := windows.Filetime{
			LowDateTime:  uint32(info.qwLastSyncTicks),
			HighDateTime: uint32(info.qwLastSyncTicks >> 32),
		}
		status.LastSuccessfulSync = time.Unix(0, lastSync.Nanoseconds())
	}

	return status, nil
}
