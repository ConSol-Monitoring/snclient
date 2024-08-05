package snclient

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/mem"
)

func (l *CheckMemory) virtualMemory() (total, avail uint64, err error) {
	ex := mem.NewExWindows()
	v, err := ex.VirtualMemory()
	if err != nil {
		return 0, 0, fmt.Errorf("fetching virtual memory failed: %s", err.Error())
	}

	return v.VirtualTotal, v.VirtualAvail, nil
}
