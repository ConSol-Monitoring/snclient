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

| Argument      | Default Value                            |
| ------------- | ---------------------------------------- |
| warning       | utilization > 80                         |
| critical      | utilization > 95                         |
| empty-state   | 3 (UNKNOWN)                              |
| empty-syntax  | %(status) - No drives found              |
| top-syntax    | %(status) - \${problem_list}             |
| ok-syntax     | %(status) - All %(count) drive(s) are ok |
| detail-syntax | %(drive) %(utilization)                  |

## Check Specific Arguments

| Argument | Description                                                                                           |
| -------- | ----------------------------------------------------------------------------------------------------- |
| drive    | Name(s) of the drives to check the IO stats for ex.: c: or / .If left empty, it will check all drives |
| lookback | Lookback period for the rate calculations, given in seconds. Default: 300                             |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute        | Description                                                                              |
| ---------------- | ---------------------------------------------------------------------------------------- |
| drive            | Name(s) of the drives to check the io stats for. If left empty, it will check all drives |
| lookback         | Lookback period for which the rate was calculated                                        |
| label            | Label of the drive                                                                       |
| read_count       | Total number of read operations completed successfully                                   |
| read_count_rate  | Number of read operations per second during the lookback period                          |
| read_bytes       | Total number of bytes read from the disk                                                 |
| read_bytes_rate  | Average bytes read per second during the lookback period                                 |
| read_time        | Total time spent on read operations (milliseconds)                                       |
| write_count      | Total number of write operations completed successfully                                  |
| write_count_rate | Number of write operations per second during the lookback period                         |
| write_bytes      | Total number of bytes written to the disk                                                |
| write_bytes_rate | Average bytes written per second during the lookback period                              |
| write_time       | Total time spent on write operations (milliseconds)                                      |
| iops_in_progress | Number of I/O operations currently in flight                                             |
| io_time          | Total time during which the disk had at least one active I/O (milliseconds)              |
| io_time_rate     | Change in I/O time per second                                                            |
| weighted_io      | Measure of both I/O completion time and the number of backlogged requests                |
| utilization      | Percentage of time the disk was busy (0-100%)                                            |
