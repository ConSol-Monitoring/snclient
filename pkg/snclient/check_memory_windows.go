package snclient

import (
	"fmt"
	"unsafe"

	"github.com/shirou/gopsutil/v4/mem"
	"golang.org/x/sys/windows"
)

var (
	modPsapi               = windows.NewLazyDLL("psapi.dll")
	procGetPerformanceInfo = modPsapi.NewProc("GetPerformanceInfo")
)

// see https://learn.microsoft.com/en-us/windows/win32/api/psapi/nf-psapi-getperformanceinfo
type performanceInformation struct {
	cb                uint32
	commitTotal       uint64
	commitLimit       uint64
	commitPeak        uint64
	physicalTotal     uint64
	physicalAvailable uint64
	systemCache       uint64
	kernelTotal       uint64
	kernelPaged       uint64
	kernelNonpaged    uint64
	pageSize          uint64
	handleCount       uint32
	processCount      uint32
	threadCount       uint32
}

func (l *CheckMemory) committedMemory() (total, avail uint64, err error) {
	// Get total memory from performance information
	var perfInfo performanceInformation
	perfInfo.cb = uint32(unsafe.Sizeof(perfInfo))
	res, _, _ := procGetPerformanceInfo.Call(uintptr(unsafe.Pointer(&perfInfo)), uintptr(perfInfo.cb))
	if res == 0 {
		err = windows.GetLastError()
		if err != nil {
			return 0, 0, fmt.Errorf("fetching committed memory failed: %s", err.Error())
		}

		return 0, 0, fmt.Errorf("fetching committed memory failed: unknown error")
	}

	used := perfInfo.commitTotal * perfInfo.pageSize
	total = perfInfo.commitLimit * perfInfo.pageSize

	return total, total - used, nil
}

func (l *CheckMemory) virtualMemory() (total, avail uint64, err error) {
	ex := mem.NewExWindows()
	v, err := ex.VirtualMemory()
	if err != nil {
		return 0, 0, fmt.Errorf("fetching virtual memory failed: %s", err.Error())
	}

	return v.VirtualTotal, v.VirtualAvail, nil
}
