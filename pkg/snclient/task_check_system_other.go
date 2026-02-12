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
			if !diskEligibleForWatch(diskName) {
				log.Tracef("not adding disk stat counter since it is not found to be eligible for watch: %s", diskName)

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

	for diskName := range diskIOCounters {
		if !diskEligibleForWatch(diskName) {
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
		c.snc.counterCreate("memory", "swp_in", c.bufferLength, c.metricsInterval)
		c.snc.counterCreate("memory", "swp_out", c.bufferLength, c.metricsInterval)
	}

	swap, err := mem.SwapMemory()
	if err != nil {
		return
	}

	c.snc.Counter.Set("memory", "swp_in", float64(swap.Sin))
	c.snc.Counter.Set("memory", "swp_out", float64(swap.Sout))
}
