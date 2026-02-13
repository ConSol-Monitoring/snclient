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
	defaultLookbackDriveIO = int64(300)
)

func NewCheckDriveIO() CheckHandler {
	return &CheckDriveIO{
		drives:   []string{},
		lookback: defaultLookbackDriveIO,
	}
}

func (l *CheckDriveIO) Build() *CheckData {
	return &CheckData{
		name:         "check_drive_io",
		description:  "Checks the disk Input / Output on the host.",
		implemented:  ALL,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"drive": {value: &l.drives, isFilter: true, description: "Name(s) of the drives to check the IO stats for ex.: c: or / .If left empty, it will check all drives"},
			"lookback": {
				value: &l.lookback, isFilter: true,
				description: fmt.Sprintf("Lookback period for value change rate and utilization calculations, given in seconds. Default: %d", defaultLookbackDriveIO),
			},
		},
		defaultWarning:  "utilization > 95",
		defaultCritical: "",
		okSyntax:        "%(status) - %(list)",
		detailSyntax:    "%(drive){{ IF label ne '' }} (%(label)){{ END }} >%(write_bytes_rate_humanized) <%(read_bytes_rate_humanized) %(utilization)%",
		topSyntax:       "%(status) - %(list)",
		emptyState:      CheckExitUnknown,
		emptySyntax:     "%(status) - No drives found",
		attributes: []CheckAttribute{
			{name: "drive", description: "Name(s) of the drives to check the io stats for. If left empty, it will check all drives. " +
				"For Windows this is the drive letter. For UNIX it is the logical name of the drive."},
			{name: "lookback", description: "Lookback period for which the value change rate and utilization is calculated."},
			{name: "read_count", description: "Total number of read operations completed successfully"},
			{name: "read_count_rate", description: "Number of read operations per second during the lookback period"},
			{name: "read_bytes", description: "Total number of bytes read from the disk", unit: UByte},
			{name: "read_bytes_rate", description: "Average bytes read per second during the lookback period", unit: UByte},
			{name: "read_bytes_rate_humanized", description: "Average bytes read per second during the lookback period, written in humanized format"},
			{name: "read_time", description: "Total time spent on read operations (milliseconds)."},
			{name: "write_count", description: "Total number of write operations completed successfully"},
			{name: "write_count_rate", description: "Number of write operations per second during the lookback period"},
			{name: "write_bytes", description: "Total number of bytes written to the disk", unit: UByte},
			{name: "write_bytes_rate", description: "Average bytes written per second during the lookback period", unit: UByte},
			{name: "write_bytes_rate_humanized", description: "Average bytes read per second during the lookback period, written in humanized format"},
			{name: "write_time", description: "Total time spent on write operations (milliseconds)."},

			// Windows does not report these

			{name: "label", description: "Label of the drive. Windows does not report this."},
			{name: "io_time", description: "Total time during which the disk had at least one active I/O (milliseconds). Windows does not report this."},
			{name: "io_time_rate", description: "Change in I/O time per second. Windows does not report this."},
			{name: "weighted_io", description: "Measure of both I/O completion time and the number of backlogged requests. Windows does not report this."},
			{name: "utilization", description: "Percentage of time the disk was busy (0-100%). Windows does not report this."},
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
	OK - C >20.1 MiB/s <1.4 GiB/s 41.2% |'C_read_count'=4791920c;;;0; 'C_read_bytes'=1729039767552c;;;0; 'C_read_time'=119710.31709ms;;;0; 'C_write_count'=2260624c;;;0; 'C_write_bytes'=479384686592c;;;0; 'C_write_time'=89071.67515ms;;;0; 'C
_utilization'=41.2%;95;;0; 'C_queue_depth'=0;;;0;

Check a UNIX drive and alert if for the last 30 seconds written bytes/second is above 10 Mb/s . Dm-0 is the name of the encrypted volume, it could be nvme0n1 or sdb as well

    check_drivesize lookback=30 warn="write_bytes_rate > 10Mb"
	OK - dm-0 >1.8 MiB/s/s <815.0 B/s/s 0.0%, sda1 >0 B/s/s <0 B/s/s 0.0% |'dm-0_read_count'=396738362c;;;0; 'dm-0_read_bytes'=33871348975616c;;;0; 'dm-0_read_time'=2990729692ms;;;0; 'dm-0_write_count'=624158141c;;;0; 'dm-0_write_bytes'=36083702012416c;;;0; 'dm-0_write_time'=1412729952ms;;;0; 'dm-0_utilization'=0%;95;;0; 'dm-0_io_time'=738627512ms;;;0; 'dm-0_weighted_io'=108492348;;;0; 'dm-0_iops_in_progress'=0;;;0; 'sda1_read_count'=3178c;;;0; 'sda1_read_bytes'=67430400c;;;0; 'sda1_read_time'=13572ms;;;0; 'sda1_write_count'=3193c;;;0; 'sda1_write_bytes'=282012672c;;;0; 'sda1_write_time'=9446ms;;;0; 'sda1_utilization'=0%;95;;0; 'sda1_io_time'=10832ms;;;0; 'sda1_weighted_io'=23019;;;0; 'sda1_iops_in_progress'=0;;;0;
	`,
		exampleArgs: "",
	}
}

func (l *CheckDriveIO) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	enabled, _, _ := snc.config.Section("/modules").GetBool("CheckDriveIO")
	if !enabled {
		return nil, fmt.Errorf("module CheckDriveIO is not enabled in /modules section")
	}

	drivesToCheck := append([]string{}, l.drives...)

	if len(l.drives) == 0 {
		// This value is calculated during the initialization of TaskSystemCheck
		drivesToCheck = append(drivesToCheck, StorageDevicesToWatch...)
	}

	sort.Strings(drivesToCheck)

	diskIOCounters, err := getIOCounters(ctx)
	if err != nil {
		return nil, fmt.Errorf("error when getting disk io counters: %s", err.Error())
	}

	partitions, err := disk.PartitionsWithContext(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("error when getting disk partitions: %s", err.Error())
	}

	for _, drive := range drivesToCheck {
		entry, err := l.buildDriveIOEntry(snc, check, drive, diskIOCounters, partitions)
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
func (l *CheckDriveIO) buildDriveIOEntry(snc *Agent, _ *CheckData, drive string, diskIOCounters any, partitions []disk.PartitionStat) (map[string]string, error) {
	entry := map[string]string{}
	entry["drive"] = drive
	entry["lookback"] = fmt.Sprintf("%d", l.lookback)

	if !diskEligibleForWatch(drive) {
		errorString := fmt.Sprintf("Drive that was passed does not seem to be eligible for watching: %s", drive)
		log.Debug(errorString)
		entry["_error"] = errorString
		entry["_skip"] = "1"

		return entry, errors.New(errorString)
	}

	deviceLogicalNameOrLetter := cleanupDriveName(drive)
	// overwrite with the cleaner name
	entry["drive"] = deviceLogicalNameOrLetter

	builtEntry := l.buildEntry(snc, diskIOCounters, deviceLogicalNameOrLetter, entry, partitions)

	if !builtEntry {
		errorString := fmt.Sprintf("DiskIOCounters did not have drive: %s", drive)
		log.Debug(errorString)
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
		log.Debug(errorString)
		entry["_error"] = entry["_error"] + " . " + errorString

		return
	}

	rate, err := counterInstance.GetRate(time.Duration(l.lookback) * time.Second)
	if err != nil {
		log.Debugf("Error when getting the counter rate, lookback: %d, counterCategory: %s, counterKey: %s, err: %s", l.lookback, counterCategory, counterKey, err.Error())
	}

	entry[entryKey] = fmt.Sprintf("%f", rate)
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
