package snclient

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/counter"
	"github.com/shirou/gopsutil/v4/disk"
)

func init() {
	AvailableChecks["check_drive_io"] = CheckEntry{"check_drive_io", NewCheckDriveIO}
}

type CheckDriveIO struct {
	drives   []string
	lookback int64
}

const (
	defaultLookback = int64(300)
)

func NewCheckDriveIO() CheckHandler {
	return &CheckDriveIO{
		drives:   []string{},
		lookback: defaultLookback,
	}
}

func (l *CheckDriveIO) Build() *CheckData {
	return &CheckData{
		name:         "check_drive_io",
		description:  "Checks the disk IO on the host.",
		implemented:  ALL,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"drive":    {value: &l.drives, isFilter: true, description: "Name(s) of the drives to check the IO stats for ex.: c: or / .If left empty, it will check all drives"},
			"lookback": {value: &l.lookback, isFilter: true, description: "Lookback period for value change rate and utilization calculations, given in seconds. Default: 300"},
		},
		defaultWarning:  "utilization > 95",
		defaultCritical: "",
		okSyntax:        "%(status) - All %(count) drive(s) are ok",
		detailSyntax:    "%(drive) %(utilization)",
		topSyntax:       "%(status) - ${problem_list}",
		emptyState:      CheckExitUnknown,
		emptySyntax:     "%(status) - No drives found",
		attributes: []CheckAttribute{
			{name: "drive", description: "Name(s) of the drives to check the io stats for. If left empty, it will check all drives. For Windows this is the drive letter. For UNIX it is the logical name of the drive."},
			{name: "lookback", description: "Lookback period for which the value change rate and utilization is calculated."},
			{name: "read_count", description: "Total number of read operations completed successfully"},
			{name: "read_count_rate", description: "Number of read operations per second during the lookback period"},
			{name: "read_bytes", description: "Total number of bytes read from the disk"},
			{name: "read_bytes_rate", description: "Average bytes read per second during the lookback period"},
			{name: "read_time", description: "Total time spent on read operations (milliseconds)"},
			{name: "write_count", description: "Total number of write operations completed successfully"},
			{name: "write_count_rate", description: "Number of write operations per second during the lookback period"},
			{name: "write_bytes", description: "Total number of bytes written to the disk"},
			{name: "write_bytes_rate", description: "Average bytes written per second during the lookback period"},
			{name: "write_time", description: "Total time spent on write operations (milliseconds)"},

			// Windows does not report these

			{name: "label", description: "Label of the drive"},
			{name: "io_time", description: "Total time during which the disk had at least one active I/O (milliseconds). Windows does not report this."},
			{name: "io_time_rate", description: "Change in I/O time per second. Windows does not report this."},
			{name: "weighted_io", description: "Measure of both I/O completion time and the number of backlogged requests. Windows does not report this."},
			{name: "utilization", description: "Percentage of time the disk was busy (0-100%).. Windows does not report this."},
			{name: "iops_in_progress", description: "Number of I/O operations currently in flight. Windows does not report this."},

			// Windows specific
			// https://learn.microsoft.com/en-us/windows/win32/api/winioctl/ns-winioctl-disk_performance

			{name: "idle_time", description: "Count of the 100 ns periods the disk was idle. Windows only"},
			{name: "query_time", description: "The time the performance query was sent. Count of 100 ns periods since the Win32 epoch of 01.01.1601. Windows only"},
			{name: "queue_depth", description: "The depth of the IO queue. Windows only."},
			{name: "split_count", description: "The cumulative count of IOs that are associated IOs. Windows only."},

			// note to future: currently only the utilization is calculated by adding io_time to the counter
			// if more stats are saved in the counters, one can calculate relatively useful metrics as well

			// utilization: delta(io_time) / delta(time)

			// avg read latency: delta(read_time) / delta(time)
			// this can indicate that a drive is failing, but depends on the type of the drive heavily
			// nvme and sata drives have much lower read latencies

			// saturation: delta(weighted_io) / delta(time)
			// indicates the average number of requests that waited in the queue. If the saturation is aways above > 1.0 the disk is fully utilized
			// if its always above > 5 the disk could not keep up with the requests, and a 5x faster disk at processing io would be necessary to keep up
		},
		exampleDefault: "",
		exampleArgs:    "",
	}
}

func (l *CheckDriveIO) Check(_ context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	enabled, _, _ := snc.config.Section("/modules").GetBool("CheckDriveIO")
	if !enabled {
		return nil, fmt.Errorf("module CheckDriveIO is not enabled in /modules section")
	}

	drivesToCheck := append([]string{}, l.drives...)

	if len(l.drives) == 0 {
		// This value is calculated during the initialization of TaskSystemCheck
		drivesToCheck = append(drivesToCheck, PartitionDevicesToWatch...)
	}

	sort.Strings(drivesToCheck)

	var diskIOCounters any
	var err error
	switch runtime.GOOS {
	case "windows":
		diskIOCounters, err = IoCountersWindows()
	default:
		diskIOCounters, err = disk.IOCounters()
	}

	if err != nil {
		return nil, fmt.Errorf("error when getting disk io counters: %s", err.Error())
	}

	for _, drive := range drivesToCheck {
		entry, err := l.buildDriveIOEntry(snc, check, drive, diskIOCounters)

		// if one entry has errors, skip it
		if err != nil {
			continue
		}

		check.listData = append(check.listData, entry)
	}

	l.addMetrics(check)

	return check.Finalize()
}

// drive parameter should be the logical name for linux, e.g 'sda'
// for windows it should be the drive letter, e.g 'C'
// diskIOCounters should either be of type: map[string]gopsutil.disk.IOCountersStat or map[string]IoCountersStatWindows
func (l *CheckDriveIO) buildDriveIOEntry(snc *Agent, _ *CheckData, drive string, diskIOCounters any) (map[string]string, error) {
	entry := map[string]string{}
	entry["drive"] = drive
	entry["lookback"] = fmt.Sprintf("%d", l.lookback)

	if !DiskEligibleForWatch(drive) {
		errorString := fmt.Sprintf("Drive that was passed does not seem to be eligible for watching: %s", drive)
		log.Debugf(errorString)
		entry["_error"] = errorString
		entry["_skip"] = "1"

		return entry, errors.New(errorString)
	}

	foundDisk := false
	deviceLogicalNameOrLetter := drive

	// adjust the drive name if parameters are not passed properly
	switch runtime.GOOS {
	case "freebsd", "darwin", "linux":
		if strings.HasPrefix(drive, "/dev/") {
			deviceLogicalNameOrLetter, _ = strings.CutPrefix(drive, "/dev/")
		}
	case "windows":
		deviceLogicalNameOrLetter, _ = strings.CutSuffix(deviceLogicalNameOrLetter, "\\")
		deviceLogicalNameOrLetter, _ = strings.CutSuffix(deviceLogicalNameOrLetter, ":")
	default:
	}

	if diskIOCounters, ok := diskIOCounters.(map[string]disk.IOCountersStat); ok {
		if counters, ok := diskIOCounters[deviceLogicalNameOrLetter]; ok {
			foundDisk = true

			// counters use this format when saving metrics
			// found in CheckSystemHandler.addLinuxDiskStats
			counterCategory := "disk_" + counters.Name

			entry["label"] = counters.Label

			entry["read_count"] = fmt.Sprintf("%d", counters.ReadCount)
			l.addRateToEntry(snc, entry, "read_count_rate", counterCategory, "read_count")
			entry["read_bytes"] = fmt.Sprintf("%d", counters.ReadBytes)
			l.addRateToEntry(snc, entry, "read_bytes_rate", counterCategory, "read_bytes")
			entry["read_time"] = fmt.Sprintf("%d", counters.ReadTime)

			entry["write_count"] = fmt.Sprintf("%d", counters.WriteCount)
			l.addRateToEntry(snc, entry, "write_count_rate", counterCategory, "write_count")
			entry["write_bytes"] = fmt.Sprintf("%d", counters.WriteBytes)
			l.addRateToEntry(snc, entry, "write_bytes_rate", counterCategory, "write_bytes")
			entry["write_time"] = fmt.Sprintf("%d", counters.WriteTime)

			entry["io_time"] = fmt.Sprintf("%d", counters.IoTime)
			l.addRateToEntry(snc, entry, "io_time_rate", counterCategory, "io_time")
			l.addUtilizationFromIoTime(entry)

			entry["iops_in_progress"] = fmt.Sprintf("%d", counters.IopsInProgress)
			entry["weighted_io"] = fmt.Sprintf("%d", counters.WeightedIO)
		}
	}

	if diskIOCounters, ok := diskIOCounters.(map[string]IOCountersStatWindows); ok {
		if counters, ok := diskIOCounters[deviceLogicalNameOrLetter]; ok {
			foundDisk = true

			// counters use this format when saving metrics
			// found in CheckSystemHandler.addLinuxDiskStats
			counterCategory := "disk_" + counters.Name

			entry["read_count"] = fmt.Sprintf("%d", counters.ReadCount)
			l.addRateToEntry(snc, entry, "read_count_rate", counterCategory, "read_count")
			entry["read_bytes"] = fmt.Sprintf("%d", counters.ReadBytes)
			l.addRateToEntry(snc, entry, "read_bytes_rate", counterCategory, "read_bytes")
			entry["read_time"] = fmt.Sprintf("%d", counters.ReadTime)

			entry["write_count"] = fmt.Sprintf("%d", counters.WriteCount)
			l.addRateToEntry(snc, entry, "write_count_rate", counterCategory, "write_count")
			entry["write_bytes"] = fmt.Sprintf("%d", counters.WriteBytes)
			l.addRateToEntry(snc, entry, "write_bytes_rate", counterCategory, "write_bytes")
			entry["write_time"] = fmt.Sprintf("%d", counters.WriteTime)

			entry["idle_time"] = fmt.Sprintf("%d", counters.IdleTime)
			entry["query_time"] = fmt.Sprintf("%d", counters.QueryTime)

			idleTimeCounter := snc.Counter.Get(counterCategory, "idle_time")
			queryTimeCounter := snc.Counter.Get(counterCategory, "query_time")

			if idleTimeCounter != nil && queryTimeCounter != nil {
				utilization, err := l.calculateUtilizationFromIdleAndQueryCounters(idleTimeCounter, queryTimeCounter)
				if err != nil {
					log.Tracef("Error when calculating utilization from IdleTime and QueryTime counters: %e", err.Error())
				}
				entry["utilization"] = fmt.Sprintf("%.1f", utilization)
			}

			entry["queue_depth"] = fmt.Sprintf("%d", counters.QueueDepth)
			entry["split_count"] = fmt.Sprintf("%d", counters.SplitCount)
		}
	}

	if !foundDisk {
		errorString := fmt.Sprintf("DiskIOCounters did not have drive: %s", drive)
		log.Debugf(errorString)
		entry["_error"] = errorString
		entry["_skip"] = "1"

		return entry, errors.New(errorString)
	}

	return entry, nil
}

func (l *CheckDriveIO) addRateToEntry(snc *Agent, entry map[string]string, entryKey, counterCategory, counterKey string) {
	counter := snc.Counter.Get(counterCategory, counterKey)
	if counter == nil {
		errorString := fmt.Sprintf("No counter found with category: %s, key: %s", counterCategory, counterKey)
		log.Debugf(errorString)
		entry["_error"] = entry["_error"] + " . " + errorString

		return
	}

	rate, err := counter.GetRate(time.Duration(l.lookback) * time.Second)
	if err != nil {
		errorString := fmt.Sprintf("Error when getting the counter rate, lookback: %d, counterCategory: %s, counterKey: %s, err: %s", l.lookback, counterCategory, counterKey, err.Error())
		log.Debugf(errorString)
		entry["_error"] = entry["_error"] + " . " + errorString

		return
	}

	entry[entryKey] = fmt.Sprintf("%v", rate)
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

	idleTimeDelta := convert.Float64(idleTimeLastPtr.Value) - convert.Float64(idleTimeLookbackPtr.Value)

	queryTimeLastPtr := queryTimeCounter.GetLast()
	queryTimeLookbackPtr := queryTimeCounter.GetAt(lookbackTime)

	if queryTimeLastPtr == nil {
		return 0, fmt.Errorf("idleTimeLastPtr is nil")
	}

	if queryTimeLookbackPtr == nil {
		return 0, fmt.Errorf("idleTimeLookbackPtr is nil")
	}

	queryTimeDelta := queryTimeLastPtr.Value.(uint64) - queryTimeLookbackPtr.Value.(uint64)

	if queryTimeDelta == 0 {
		return 0, fmt.Errorf("queryTimeDelta is 0, calculation will result in NaN")
	}

	idleRatio := idleTimeDelta / float64(queryTimeDelta)

	utilizationRatio = 1 - idleRatio

	utilizationPercentage := utilizationRatio * 100

	return utilizationPercentage, nil
}

func (l *CheckDriveIO) addMetrics(check *CheckData) {
	needReadCountRate := check.HasThreshold("read_count_rate")
	needReadBytesRate := check.HasThreshold("read_bytes_rate")
	needWriteCountRate := check.HasThreshold("write_count_rate")
	needWriteBytesRate := check.HasThreshold("write_bytes_rate")
	needIoTimeRate := check.HasThreshold("io_time_rate")

	for _, data := range check.listData {
		if needReadCountRate {
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					Name:          data["drive"] + "read_count_rate",
					ThresholdName: "read_count_rate",
					Value:         convert.Float64(data["read_count_rate"]),
					Unit:          "",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				},
			)
		}

		if needReadBytesRate {
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					Name:          data["drive"] + " read_bytes_rate",
					ThresholdName: "read_bytes_rate",
					Value:         convert.Float64(data["read_bytes_rate"]),
					Unit:          "B",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				},
			)
		}

		if needWriteCountRate {
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					Name:          data["drive"] + " write_count_rate",
					ThresholdName: "write_count_rate",
					Value:         convert.Float64(data["write_count_rate"]),
					Unit:          "",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				},
			)
		}

		if needWriteBytesRate {
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					Name:          data["drive"] + " write_bytes_rate",
					ThresholdName: "write_bytes_rates",
					Value:         convert.Float64(data["write_bytes_rate"]),
					Unit:          "B",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				},
			)
		}

		if needIoTimeRate {
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					Name:          data["drive"] + " io_time_rate",
					ThresholdName: "io_time_rate",
					Value:         convert.Float64(data["io_time_rate"]),
					Unit:          "",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				},
			)
		}
	}
}
