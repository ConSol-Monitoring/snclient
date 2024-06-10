package snclient

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/mem"
)

func (l *CheckMemory) virtualMemory() (total, avail uint64, err error) {
	ex := mem.NewExWindows()
	v, err := ex.VirtualMemory()

	return v.VirtualTotal, v.VirtualAvail, fmt.Errorf("fetching virtual memory failed: %s", err.Error())
}
