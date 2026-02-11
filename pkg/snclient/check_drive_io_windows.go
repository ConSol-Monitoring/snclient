package snclient

import (
	"fmt"
	"time"

	"github.com/consol-monitoring/snclient/pkg/counter"
)

func getIOCounters() (any, error) {
	return IoCountersWindows()
}

func (l *CheckDriveIO) buildEntry(snc *Agent, diskIOCounters any, deviceLogicalNameOrLetter string, entry map[string]string) (foundDisk bool) {
	diskIOCountersTypecasted, ok := diskIOCounters.(map[string]IOCountersStatWindows)
	if !ok {
		log.Debug("Platform is windows, diskIOCounters should have IOCountersStatWindows keys")
		return false
	}

	if counters, ok := diskIOCountersTypecasted[deviceLogicalNameOrLetter]; ok {
		foundDisk = true

		// counters use this format when saving metrics
		// found in CheckSystemHandler.addLinuxDiskStats
		counterCategory := "disk_" + counters.Name

		entry["read_count"] = fmt.Sprintf("%d", counters.ReadCount)
		l.addRateToEntry(snc, entry, "read_count_rate", counterCategory, "read_count")
		entry["read_bytes"] = fmt.Sprintf("%d", counters.ReadBytes)
		l.addRateToEntry(snc, entry, "read_bytes_rate", counterCategory, "read_bytes")
		entry["read_time"] = fmt.Sprintf("%f", counters.ReadTime)

		entry["write_count"] = fmt.Sprintf("%d", counters.WriteCount)
		l.addRateToEntry(snc, entry, "write_count_rate", counterCategory, "write_count")
		entry["write_bytes"] = fmt.Sprintf("%d", counters.WriteBytes)
		l.addRateToEntry(snc, entry, "write_bytes_rate", counterCategory, "write_bytes")
		entry["write_time"] = fmt.Sprintf("%f", counters.WriteTime)

		entry["idle_time"] = fmt.Sprintf("%d", counters.IdleTime)
		entry["query_time"] = fmt.Sprintf("%d", counters.QueryTime)

		idleTimeCounter := snc.Counter.Get(counterCategory, "idle_time")
		queryTimeCounter := snc.Counter.Get(counterCategory, "query_time")

		if idleTimeCounter != nil && queryTimeCounter != nil {
			utilization, err := l.calculateUtilizationFromIdleAndQueryCounters(idleTimeCounter, queryTimeCounter)
			if err != nil {
				log.Tracef("Error when calculating utilization from IdleTime and QueryTime counters: %s", err.Error())
			}
			entry["utilization"] = fmt.Sprintf("%.1f", utilization)
		}

		entry["queue_depth"] = fmt.Sprintf("%d", counters.QueueDepth)
		entry["split_count"] = fmt.Sprintf("%d", counters.SplitCount)
	}

	return foundDisk
}

func (l *CheckDriveIO) calculateUtilizationFromIdleAndQueryCounters(idleTimeCounter, queryTimeCounter *counter.Counter) (utilizationRatio float64, err error) {
	// This function is designed for windows.
	// Windows does not expose an io time, instead it counts idle time
	// Each query for disk performance additionally returns a query time

	// delta(idle time) / delta(query time) -> returns the idle ratio
	// 1 - idle ratio ~ utilization ratio

	lookbackTime := time.Now().Add(-time.Duration(l.lookback) * time.Second)

	idleTimeLastPtr := idleTimeCounter.GetLast()
	idleTimeLookbackPtr := idleTimeCounter.GetAt(lookbackTime)

	if idleTimeLastPtr == nil {
		return 0, fmt.Errorf("idleTimeLastPtr is nil")
	}

	if idleTimeLookbackPtr == nil {
		return 0, fmt.Errorf("idleTimeLookbackPtr is nil")
	}

	idleTimeLastUint64, ok := idleTimeLastPtr.Value.(uint64)
	if !ok {
		return 0, fmt.Errorf("idleTimeLastPtr.Value is not uint64")
	}

	idleTimeLookbackUint64, ok := idleTimeLookbackPtr.Value.(uint64)
	if !ok {
		return 0, fmt.Errorf("idleTimeLookbackPtr.Value is not uint64")
	}

	idleTimeDelta := idleTimeLastUint64 - idleTimeLookbackUint64

	// idle time may be 0 for real if drive did nothing, no need to check it unlike queryTime

	if idleTimeLastUint64 < idleTimeLookbackUint64 {
		return 0, fmt.Errorf("idleTimeLastUint64 is smaller than idleTimeLookbackUint64, it is a counter and should always grow.")
	}

	queryTimeLastPtr := queryTimeCounter.GetLast()
	queryTimeLookbackPtr := queryTimeCounter.GetAt(lookbackTime)

	if queryTimeLastPtr == nil {
		return 0, fmt.Errorf("queryTimeLastPtr is nil")
	}

	if queryTimeLookbackPtr == nil {
		return 0, fmt.Errorf("queryTimeLookbackPtr is nil")
	}

	queryTimeLastUint64, ok := queryTimeLastPtr.Value.(uint64)
	if !ok {
		return 0, fmt.Errorf("queryTimeLastPtr.Value is not uint64")
	}

	queryTimeLookbackUint64, ok := queryTimeLookbackPtr.Value.(uint64)
	if !ok {
		return 0, fmt.Errorf("queryTimeLookbackPtr.Value is not uint64")
	}

	queryTimeDelta := queryTimeLastUint64 - queryTimeLookbackUint64

	if queryTimeDelta == 0 {
		return 0, fmt.Errorf("queryTimeDelta is 0, calculation will result in NaN")
	}

	if queryTimeLastUint64 < queryTimeLookbackUint64 {
		return 0, fmt.Errorf("queryTimeLastUint64 is smaller than queryTimeLookbackUint64, it is a timestamp counter and should always grow.")
	}

	// This may not be precise, but there are no other primitives to do it
	idleRatio := float64(idleTimeDelta) / float64(queryTimeDelta)

	utilizationRatio = 1 - idleRatio

	utilizationPercentage := utilizationRatio * 100

	return utilizationPercentage, nil
}
