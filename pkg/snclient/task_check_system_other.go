//go:build !windows

package snclient

import (
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

// Unix uses the function from gopsutil disk.IOCounters()
// The function reports these attributes correctly
func (c *CheckSystemHandler) addDiskStats(create bool) {
	diskIOCounters, err := disk.IOCounters()
	// do not create the counters if there is an error
	if err != nil {
		return
	}

	if create {
		for diskName := range diskIOCounters {
			if !DiskEligibleForWatch(diskName) {
				log.Tracef("not adding disk stat counter since it is found to be not-physical: %s", diskName)

				continue
			}

			category := "disk_" + diskName
			c.snc.counterCreate(category, "write_bytes", c.bufferLength, c.metricsInterval)
			c.snc.counterCreate(category, "write_count", c.bufferLength, c.metricsInterval)
			c.snc.counterCreate(category, "read_bytes", c.bufferLength, c.metricsInterval)
			c.snc.counterCreate(category, "read_count", c.bufferLength, c.metricsInterval)
			c.snc.counterCreate(category, "io_time", c.bufferLength, c.metricsInterval)
		}
	}

	// use the no-copy range iteraiton, otherwise we copy the whole struct and linter does not allow it
	for diskName := range diskIOCounters {
		if !DiskEligibleForWatch(diskName) {
			continue
		}

		category := "disk_" + diskName

		c.snc.Counter.Set(category, "write_bytes", float64(diskIOCounters[diskName].WriteBytes))
		c.snc.Counter.Set(category, "write_count", float64(diskIOCounters[diskName].WriteCount))
		c.snc.Counter.Set(category, "read_bytes", float64(diskIOCounters[diskName].ReadBytes))
		c.snc.Counter.Set(category, "read_count", float64(diskIOCounters[diskName].ReadCount))
		c.snc.Counter.Set(category, "io_time", float64(diskIOCounters[diskName].IoTime))
	}
}

func (c *CheckSystemHandler) addMemoryStats(create bool) {
	if create {
		c.snc.counterCreate("memory", "total", c.bufferLength, c.metricsInterval)
		c.snc.counterCreate("memory", "used", c.bufferLength, c.metricsInterval)
		c.snc.counterCreate("memory", "swp_in", c.bufferLength, c.metricsInterval)
		c.snc.counterCreate("memory", "swp_out", c.bufferLength, c.metricsInterval)
	}

	virtualMemory, err := mem.VirtualMemory()
	if err != nil {
		return
	}

	swap, err := mem.SwapMemory()
	if err != nil {
		return
	}

	c.snc.Counter.Set("memory", "total", float64(virtualMemory.Total))
	c.snc.Counter.Set("memory", "used", float64(virtualMemory.Used))
	c.snc.Counter.Set("memory", "swp_in", float64(swap.Sin))
	c.snc.Counter.Set("memory", "swp_out", float64(swap.Sout))
}
