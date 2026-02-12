---
title: cpu
---

## check_cpu

Checks the cpu usage metrics.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_cpu
    OK - CPU load is ok. |'total 5m'=13%;80;90 'total 1m'=13%;80;90 'total 5s'=13%;80;90

Checking **each core** by adding filter=none (disabling the filter):

    check_cpu filter=none
    OK - CPU load is ok. |'core1 5m'=13%;80;90 'core1 1m'=12%;80;90 'core1 5s'=9%;80;90...

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_cpu
        use                  generic-service
        check_command        check_nrpe!check_cpu!'warn=load > 80' 'crit=load > 95'
    }

## Argument Defaults

| Argument      | Default Value                                                     |
| ------------- | ----------------------------------------------------------------- |
| filter        | core = 'total'                                                    |
| warning       | load > 80                                                         |
| critical      | load > 90                                                         |
| empty-state   | 3 (UNKNOWN)                                                       |
| empty-syntax  | check_cpu failed to find anything with this filter.               |
| top-syntax    | %(status) - \${problem_list} on %{core_num} cores                 |
| ok-syntax     | %(status) - CPU load is ok. %{total:fmt=%d}% on %{core_num} cores |
| detail-syntax | \${time}: \${load}%                                               |

## Check Specific Arguments

| Argument         | Description                                                           |
| ---------------- | --------------------------------------------------------------------- |
| n\|procs-to-show | Number of processes to show when printing the top consuming processes |
| show-args        | Show arguments when listing the top N processes                       |
| time             | The times to check, default: 5m,1m,5s                                 |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Description                                                    |
| --------- | -------------------------------------------------------------- |
| core      | Core to check (total or core ##)                               |
| core_id   | Core to check (total or core_##)                               |
| idle      | Current idle load for a given core (currently not supported)   |
| kernel    | Current kernel load for a given core (currently not supported) |
| load      | Current load for a given core                                  |
| time      | Time frame checked                                             |
