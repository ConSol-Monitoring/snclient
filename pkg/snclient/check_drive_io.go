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
		okSyntax:        "%(status) - %(list)",
		detailSyntax:    "%(drive) >%(write_bytes_rate) <%(read_bytes_rate) %(utilization)",
		topSyntax:       "%(status) - %(list)",
		emptyState:      CheckExitUnknown,
		emptySyntax:     "%(status) - No drives found",
		attributes: []CheckAttribute{
			{name: "drive", description: "Name(s) of the drives to check the io stats for. If left empty, it will check all drives. " +
				"For Windows this is the drive letter. For UNIX it is the logical name of the drive."},
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
		//nolint:lll // the output is long
		exampleDefault: `
    check_drive_io
    OK - All 1 drive(s) are ok

Check a single drive IO, and show utilization details

    check_drive_io drive='C:' show-all
    OK - C: 0.2

Check a UNIX drive and alert if for the last 30 seconds written bytes/second is above 10 Mb/s . Dm-0 is the name of the encrypted volume, it could be nvme0n1 or sdb as well

    check_drivesize lookback=30 warn="write_bytes_rate > 10Mb"
	OK - dm-0 >580134.2346306148 <2621.335146594136 0.3 |'dm-0_read_count'=525328;;;0; 'dm-0_read_bytes'=19601354752B;;;0; 'dm-0_read_time'=126528;;;0; 'dm-0_write_count'=4182134;;;0; 'dm-0_write_bytes'=263957790720B;;;0; 'dm-0_write_time'=145147492;;;0; 'dm-0_utilization'=0.3;;;0; 'dm-0_io_time'=307500;;;0; 'dm-0_weighted_io'=145274020;;;0; 'dm-0_iops_in_progress'=0;;;0;
	`,
		exampleArgs: "",
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

	diskIOCounters, err := getIOCounters()
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

	deviceLogicalNameOrLetter := cleanupDriveName(drive)
	// overwrite with the cleaner name
	entry["drive"] = deviceLogicalNameOrLetter

	builtEntry := l.buildEntry(snc, diskIOCounters, deviceLogicalNameOrLetter, entry)

	if !builtEntry {
		errorString := fmt.Sprintf("DiskIOCounters did not have drive: %s", drive)
		log.Debugf(errorString)
		entry["_error"] = errorString
		entry["_skip"] = "1"

		return entry, errors.New(errorString)
	}

	return entry, nil
}

func (l *CheckDriveIO) addRateToEntry(snc *Agent, entry map[string]string, entryKey, counterCategory, counterKey string) {
	counterInstance := snc.Counter.Get(counterCategory, counterKey)
	if counterInstance == nil {
		errorString := fmt.Sprintf("No counter found with category: %s, key: %s", counterCategory, counterKey)
		log.Debugf(errorString)
		entry["_error"] = entry["_error"] + " . " + errorString

		return
	}

	rate, err := counterInstance.GetRate(time.Duration(l.lookback) * time.Second)
	if err != nil {
		errorString := fmt.Sprintf("Error when getting the counter rate, lookback: %d, counterCategory: %s, counterKey: %s, err: %s", l.lookback, counterCategory, counterKey, err.Error())
		log.Debugf(errorString)
		entry["_error"] = entry["_error"] + " . " + errorString

		return
	}

	entry[entryKey] = fmt.Sprintf("%v", rate)
}

func cleanupDriveName(drive string) (deviceLogicalNameOrLetter string) {
	deviceLogicalNameOrLetter = drive

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

	return deviceLogicalNameOrLetter
}

//nolint:funlen // it is long because there are a lot of attributes. the function is simple
func (l *CheckDriveIO) addMetrics(check *CheckData) {
	for _, data := range check.listData {
		check.result.Metrics = append(check.result.Metrics,
			&CheckMetric{
				Name:          data["drive"] + "_read_count",
				ThresholdName: "read_count",
				Value:         convert.Float64(data["read_count"]),
				Unit:          "c",
				Warning:       check.warnThreshold,
				Critical:      check.critThreshold,
				Min:           &Zero,
			},
			&CheckMetric{
				Name:          data["drive"] + "_read_bytes",
				ThresholdName: "read_bytes",
				Value:         convert.Float64(data["read_bytes"]),
				Unit:          "c",
				Warning:       check.warnThreshold,
				Critical:      check.critThreshold,
				Min:           &Zero,
			},
			&CheckMetric{
				Name:          data["drive"] + "_read_time",
				ThresholdName: "read_time",
				Value:         convert.Float64(data["read_time"]),
				Unit:          "ms",
				Warning:       check.warnThreshold,
				Critical:      check.critThreshold,
				Min:           &Zero,
			},
			&CheckMetric{
				Name:          data["drive"] + "_write_count",
				ThresholdName: "write_count",
				Value:         convert.Float64(data["write_count"]),
				Unit:          "c",
				Warning:       check.warnThreshold,
				Critical:      check.critThreshold,
				Min:           &Zero,
			},
			&CheckMetric{
				Name:          data["drive"] + "_write_bytes",
				ThresholdName: "write_bytes",
				Value:         convert.Float64(data["write_bytes"]),
				Unit:          "c",
				Warning:       check.warnThreshold,
				Critical:      check.critThreshold,
				Min:           &Zero,
			},
			&CheckMetric{
				Name:          data["drive"] + "_write_time",
				ThresholdName: "write_time",
				Value:         convert.Float64(data["write_time"]),
				Unit:          "ms",
				Warning:       check.warnThreshold,
				Critical:      check.critThreshold,
				Min:           &Zero,
			},
			&CheckMetric{
				Name:          data["drive"] + "_utilization",
				ThresholdName: "utilization",
				Value:         convert.Float64(data["utilization"]),
				Unit:          "%",
				Warning:       check.warnThreshold,
				Critical:      check.critThreshold,
				Min:           &Zero,
			},
		)

		switch runtime.GOOS {
		case "windows":
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					Name:          data["drive"] + "_queue_depth",
					ThresholdName: "queue_depth",
					Value:         convert.Float64(data["queue_depth"]),
					Unit:          "",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				},
			)
		default:
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					Name:          data["drive"] + "_io_time",
					ThresholdName: "io_time",
					Value:         convert.Float64(data["io_time"]),
					Unit:          "ms",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				},
				&CheckMetric{
					Name:          data["drive"] + "_weighted_io",
					ThresholdName: "weighted_io",
					Value:         convert.Float64(data["weighted_io"]),
					Unit:          "",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				},
				&CheckMetric{
					Name:          data["drive"] + "_iops_in_progress",
					ThresholdName: "iops_in_progress",
					Value:         convert.Float64(data["iops_in_progress"]),
					Unit:          "",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				},
			)
		}
	}
}
