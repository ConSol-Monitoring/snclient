//go:build !windows

package snclient

import (
	"context"
	"fmt"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/humanize"
	"github.com/shirou/gopsutil/v4/disk"
)

func getIOCounters(ctx context.Context) (any, error) {
	counters, err := disk.IOCountersWithContext(ctx)
	if err != nil {
		return counters, fmt.Errorf("Error when getting IO Counters: %w", err)
	}

	return counters, nil
}

func (l *CheckDriveIO) buildEntry(snc *Agent, diskIOCounters any, deviceLogicalNameOrLetter string, entry map[string]string, partitions []disk.PartitionStat) (foundDisk bool) {
	diskIOCounterStat, ok := diskIOCounters.(map[string]disk.IOCountersStat)
	if !ok {
		return false
	}

	counters, ok := diskIOCounterStat[deviceLogicalNameOrLetter]
	if !ok {
		return false
	}

	// counters use this format when saving metrics
	// found in CheckSystemHandler.addLinuxDiskStats
	counterCategory := "disk_" + counters.Name

	entry["label"] = counters.Label
	if counters.Label == "" {
		// if label is empty, we can try to find a more user friendly name from the partitions list
		for _, partition := range partitions {
			if partition.Device == counters.Name || partition.Device == "/dev/"+counters.Name {
				entry["label"] = partition.Mountpoint

				break
			}
		}
	}

	entry["read_count"] = fmt.Sprintf("%d", counters.ReadCount)
	l.addRateToEntry(snc, entry, "read_count_rate", counterCategory, "read_count")
	entry["read_bytes"] = fmt.Sprintf("%d", counters.ReadBytes)
	l.addRateToEntry(snc, entry, "read_bytes_rate", counterCategory, "read_bytes")
	readBytesRateFloat64 := convert.Float64(entry["read_bytes_rate"])
	humanizedReadBytesRate := humanize.IBytesF(uint64(readBytesRateFloat64), 1)
	entry["read_bytes_rate_humanized"] = humanizedReadBytesRate + "/s"
	entry["read_time"] = fmt.Sprintf("%d", counters.ReadTime)

	entry["write_count"] = fmt.Sprintf("%d", counters.WriteCount)
	l.addRateToEntry(snc, entry, "write_count_rate", counterCategory, "write_count")
	entry["write_bytes"] = fmt.Sprintf("%d", counters.WriteBytes)
	l.addRateToEntry(snc, entry, "write_bytes_rate", counterCategory, "write_bytes")
	writeBytesRateFloat64 := convert.Float64(entry["write_bytes_rate"])
	humanizedWriteBytesRate := humanize.IBytesF(uint64(writeBytesRateFloat64), 1)
	entry["write_bytes_rate_humanized"] = humanizedWriteBytesRate + "/s"
	entry["write_time"] = fmt.Sprintf("%d", counters.WriteTime)

	entry["io_time"] = fmt.Sprintf("%d", counters.IoTime)
	l.addRateToEntry(snc, entry, "io_time_rate", counterCategory, "io_time")
	l.addUtilizationFromIoTime(entry)

	entry["iops_in_progress"] = fmt.Sprintf("%d", counters.IopsInProgress)
	entry["weighted_io"] = fmt.Sprintf("%d", counters.WeightedIO)

	return true
}

func (l *CheckDriveIO) addUtilizationFromIoTime(entry map[string]string) {
	// io_time is most likely field 10 in /proc/diskstats on Linux
	// 		Field 10 -- # of milliseconds spent doing I/Os (unsigned int)
	// This field increases so long as field 9 is nonzero.

	// Since 5.0 this field counts jiffies when at least one request was
	// started or completed. If request runs more than 2 jiffies then some
	// I/O time might be not accounted in case of concurrent requests.

	// The documentation tells that 'it counts jiffies' , but that contradicts its own saying at the beginning
	// We found that it is most likely counting miliseconds

	// getRate function returns change / seconds, but the counter increases per millisecond

	// ex 624 miliseconds of io_time / second -> 624/1000 = 0.624 overall occupancy -> 0.624 * 100 = 62.4 percent utilization
	entry["utilization"] = fmt.Sprintf("%.1f", convert.Float64(entry["io_time_rate"])/10)
}
