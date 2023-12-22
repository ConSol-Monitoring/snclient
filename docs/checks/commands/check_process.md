---
title: process
---

## check_process

Checks the state and metrics of one or multiple processes.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_process
    OK: 417 processes. |'count'=417;;;0

Check specific process by name (adding some metrics as well)

    check_process process=httpd warn='count <= 0 || count > 10' crit='count <= 0 || count > 20' top-syntax='%{status} - %{count} processes, memory %{rss|h}B, started %{oldest:age|duration} ago'
    WARNING - 12 processes, memory 62.58 MB, started 01:11h ago |...

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_process
        use                  generic-service
        check_command        check_nrpe!check_process!warn='count <= 0 || count > 10' crit='count <= 0 || count > 20'
    }

## Argument Defaults

| Argument      | Default Value                                           |
| ------------- | ------------------------------------------------------- |
| empty-state   | 3 (UNKNOWN)                                             |
| empty-syntax  | check_process failed to find anything with this filter. |
| top-syntax    | \${status}: \${problem_list}                            |
| ok-syntax     | %(status): all %{count} processes are ok.               |
| detail-syntax | \${exe}=\${state}                                       |

## Check Specific Arguments

| Argument | Description                                                |
| -------- | ---------------------------------------------------------- |
| process  | The process to check, set to \* to check all. Default: \*  |
| timezone | Sets the timezone for time metrics (default is local time) |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute        | Description                                                                                                |
| ---------------- | ---------------------------------------------------------------------------------------------------------- |
| process          | Name of the executable (without path)                                                                      |
| exe              | Name of the executable (without path)                                                                      |
| filename         | Name of the executable with path                                                                           |
| command_line     | Full command line of process                                                                               |
| state            | Current state (windows: started, stopped, hung - linux: idle, lock, running, sleep, stop, wait and zombie) |
| creation         | Start time of process                                                                                      |
| pid              | Process id                                                                                                 |
| uid              | User if of process owner (linux only)                                                                      |
| username         | User name of process owner (linux only)                                                                    |
| cpu              | CPU usage in percent                                                                                       |
| virtual          | Virtual memory usage in bytes                                                                              |
| rss              | Resident memory usage in bytes                                                                             |
| pagefile         | Swap memory usage in bytes                                                                                 |
| oldest           | Unix timestamp of oldest process                                                                           |
| peak_pagefile    | Peak swap memory usage in bytes (windows only)                                                             |
| handles          | Number of handles (windows only)                                                                           |
| kernel           | Kernel time in seconds (windows only)                                                                      |
| peak_virtual     | Peak virtual size in bytes (windows only)                                                                  |
| peak_working_set | Peak working set in bytes (windows only)                                                                   |
| user             | User time in seconds (windows only)                                                                        |
| working_set      | Working set in bytes (windows only)                                                                        |
