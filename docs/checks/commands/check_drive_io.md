---
title: drive_io
---

## check_drive_io

Checks the disk IO on the host.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_drive_io
    OK - All 1 drive(s) are ok

Check a single drive IO, and show utilization details

    check_drive_io drive='C:' show-all
    OK - C: 0.2

Check a UNIX drive and alert if for the last 30 seconds written bytes/second is above 10 Mb/s . Dm-0 is the name of the encrypted volume, it could be nvme0n1 or sdb as well

    check_drivesize lookback=30 warn="write_bytes_rate > 10Mb"
	OK - dm-0 >580134.2346306148 <2621.335146594136 0.3 |'dm-0_read_count'=525328;;;0; 'dm-0_read_bytes'=19601354752B;;;0; 'dm-0_read_time'=126528;;;0; 'dm-0_write_count'=4182134;;;0; 'dm-0_write_bytes'=263957790720B;;;0; 'dm-0_write_time'=145147492;;;0; 'dm-0_utilization'=0.3;;;0; 'dm-0_io_time'=307500;;;0; 'dm-0_weighted_io'=145274020;;;0; 'dm-0_iops_in_progress'=0;;;0;

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_drive_io
        use                  generic-service
        check_command        check_nrpe!check_drive_io!
    }

## Argument Defaults

| Argument      | Default Value                                                    |
| ------------- | ---------------------------------------------------------------- |
| warning       | utilization > 95                                                 |
| empty-state   | 3 (UNKNOWN)                                                      |
| empty-syntax  | %(status) - No drives found                                      |
| top-syntax    | %(status) - %(list)                                              |
| ok-syntax     | %(status) - %(list)                                              |
| detail-syntax | %(drive) >%(write_bytes_rate) <%(read_bytes_rate) %(utilization) |

## Check Specific Arguments

| Argument | Description                                                                                           |
| -------- | ----------------------------------------------------------------------------------------------------- |
| drive    | Name(s) of the drives to check the IO stats for ex.: c: or / .If left empty, it will check all drives |
| lookback | Lookback period for value change rate and utilization calculations, given in seconds. Default: 300    |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute        | Description                                                                                        |
| ---------------- | -------------------------------------------------------------------------------------------------- |
| drive            | Name(s) of the drives to check the io stats for. If left empty, it will check all drives. For Windows this is the drive letter. For UNIX it is the logical name of the drive. |
| lookback         | Lookback period for which the value change rate and utilization is calculated.                     |
| read_count       | Total number of read operations completed successfully                                             |
| read_count_rate  | Number of read operations per second during the lookback period                                    |
| read_bytes       | Total number of bytes read from the disk                                                           |
| read_bytes_rate  | Average bytes read per second during the lookback period                                           |
| read_time        | Total time spent on read operations (milliseconds)                                                 |
| write_count      | Total number of write operations completed successfully                                            |
| write_count_rate | Number of write operations per second during the lookback period                                   |
| write_bytes      | Total number of bytes written to the disk                                                          |
| write_bytes_rate | Average bytes written per second during the lookback period                                        |
| write_time       | Total time spent on write operations (milliseconds)                                                |
| label            | Label of the drive                                                                                 |
| io_time          | Total time during which the disk had at least one active I/O (milliseconds). Windows does not report this. |
| io_time_rate     | Change in I/O time per second. Windows does not report this.                                       |
| weighted_io      | Measure of both I/O completion time and the number of backlogged requests. Windows does not report this. |
| utilization      | Percentage of time the disk was busy (0-100%).. Windows does not report this.                      |
| iops_in_progress | Number of I/O operations currently in flight. Windows does not report this.                        |
| idle_time        | Count of the 100 ns periods the disk was idle. Windows only                                        |
| query_time       | The time the performance query was sent. Count of 100 ns periods since the Win32 epoch of 01.01.1601. Windows only |
| queue_depth      | The depth of the IO queue. Windows only.                                                           |
| split_count      | The cumulative count of IOs that are associated IOs. Windows only.                                 |
