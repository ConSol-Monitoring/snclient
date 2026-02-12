---
title: drive_io
---

## check_drive_io

Checks the disk Input / Output on the host.

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
	OK - C >20.1 MiB/s <1.4 GiB/s 41.2% |'C_read_count'=4791920c;;;0; 'C_read_bytes'=1729039767552c;;;0; 'C_read_time'=119710.31709ms;;;0; 'C_write_count'=2260624c;;;0; 'C_write_bytes'=479384686592c;;;0; 'C_write_time'=89071.67515ms;;;0; 'C
_utilization'=41.2%;95;;0; 'C_queue_depth'=0;;;0;

Check a UNIX drive and alert if for the last 30 seconds written bytes/second is above 10 Mb/s . Dm-0 is the name of the encrypted volume, it could be nvme0n1 or sdb as well

    check_drivesize lookback=30 warn="write_bytes_rate > 10Mb"
	OK - dm-0 >1.8 MiB/s/s <815.0 B/s/s 0.0%, sda1 >0 B/s/s <0 B/s/s 0.0% |'dm-0_read_count'=396738362c;;;0; 'dm-0_read_bytes'=33871348975616c;;;0; 'dm-0_read_time'=2990729692ms;;;0; 'dm-0_write_count'=624158141c;;;0; 'dm-0_write_bytes'=36083702012416c;;;0; 'dm-0_write_time'=1412729952ms;;;0; 'dm-0_utilization'=0%;95;;0; 'dm-0_io_time'=738627512ms;;;0; 'dm-0_weighted_io'=108492348;;;0; 'dm-0_iops_in_progress'=0;;;0; 'sda1_read_count'=3178c;;;0; 'sda1_read_bytes'=67430400c;;;0; 'sda1_read_time'=13572ms;;;0; 'sda1_write_count'=3193c;;;0; 'sda1_write_bytes'=282012672c;;;0; 'sda1_write_time'=9446ms;;;0; 'sda1_utilization'=0%;95;;0; 'sda1_io_time'=10832ms;;;0; 'sda1_weighted_io'=23019;;;0; 'sda1_iops_in_progress'=0;;;0;

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

| Argument      | Default Value                                                                                         |
| ------------- | ----------------------------------------------------------------------------------------------------- |
| warning       | utilization > 95                                                                                      |
| empty-state   | 3 (UNKNOWN)                                                                                           |
| empty-syntax  | %(status) - No drives found                                                                           |
| top-syntax    | %(status) - %(list)                                                                                   |
| ok-syntax     | %(status) - %(list)                                                                                   |
| detail-syntax | %(drive){{ IF label ne '' }} (%(label)){{ END }} >%(write_bytes_rate) <%(read_bytes_rate) %(utilization)% |

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
| read_time        | Total time spent on read operations (milliseconds).                                                |
| write_count      | Total number of write operations completed successfully                                            |
| write_count_rate | Number of write operations per second during the lookback period                                   |
| write_bytes      | Total number of bytes written to the disk                                                          |
| write_bytes_rate | Average bytes written per second during the lookback period                                        |
| write_time       | Total time spent on write operations (milliseconds).                                               |
| label            | Label of the drive. Windows does not report this.                                                  |
| io_time          | Total time during which the disk had at least one active I/O (milliseconds). Windows does not report this. |
| io_time_rate     | Change in I/O time per second. Windows does not report this.                                       |
| weighted_io      | Measure of both I/O completion time and the number of backlogged requests. Windows does not report this. |
| utilization      | Percentage of time the disk was busy (0-100%). Windows does not report this.                       |
| iops_in_progress | Number of I/O operations currently in flight. Windows does not report this.                        |
| idle_time        | Count of the 100 ns periods the disk was idle. Windows only                                        |
| query_time       | The time the performance query was sent. Count of 100 ns periods since the Win32 epoch of 01.01.1601. Windows only |
| queue_depth      | The depth of the IO queue. Windows only.                                                           |
| split_count      | The cumulative count of IOs that are associated IOs. Windows only.                                 |
