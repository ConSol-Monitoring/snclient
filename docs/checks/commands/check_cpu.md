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
    OK: CPU load is ok. |'total 5m'=13%;80;90 'total 1m'=13%;80;90 'total 5s'=13%;80;90

Checking **each core** by adding filter=none (disabling the filter):

    check_cpu filter=none
    OK: CPU load is ok. |'core4 5m'=13%;80;90 'core4 1m'=12%;80;90 'core4 5s'=9%;80;90 'core6 5m'=10%;80;90 'core6 1m'=10%;80;90 'core6 5s'=3%;80;90 'core5 5m'=10%;80;90 'core5 1m'=9%;80;90 'core5 5s'=6%;80;90 'core7 5m'=10%;80;90 'core7 1m'=10%;80;90 'core7 5s'=7%;80;90 'core1 5m'=13%;80;90 'core1 1m'=12%;80;90 'core1 5s'=10%;80;90 'core2 5m'=17%;80;90 'core2 1m'=17%;80;90 'core2 5s'=9%;80;90 'total 5m'=12%;80;90 'total 1m'=12%;80;90 'total 5s'=8%;80;90 'core3 5m'=12%;80;90 'core3 1m'=12%;80;90 'core3 5s'=11%;80;90 'core0 5m'=14%;80;90 'core0 1m'=14%;80;90 'core0 5s'=14%;80;90

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name    check_nrpe
        command_line    $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name               testhost
        service_description     check_cpu
        use                     generic-service
        check_command           check_nrpe!check_cpu!'warn=load > 80' 'crit=load > 95'
    }

## Argument Defaults

| Argument      | Default Value                                       |
| ------------- | --------------------------------------------------- |
| filter        | core = 'total'                                      |
| warning       | load > 80                                           |
| critcal       | load > 90                                           |
| empty-state   | 3 (UNKNOWN)                                         |
| empty-syntax  | check_cpu failed to find anything with this filter. |
| top-syntax    | \${status}: \${problem_list}                        |
| ok-syntax     | %(status): CPU load is ok.                          |
| detail-syntax | \${time}: \${load}%                                 |

## Check Specific Arguments

| Argument | Description                           |
| -------- | ------------------------------------- |
| time     | The times to check, default: 5m,1m,5s |

## Attributes

### Check Specific Attributes

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Description                                                    |
| --------- | -------------------------------------------------------------- |
| core      | Core to check (total or core ##)                               |
| core_id   | Core to check (total or core_##)                               |
| idle      | Current idle load for a given core (currently not supported)   |
| kernel    | Current kernel load for a given core (currently not supported) |
| load      | Current load for a given core                                  |
| time      | Time frame to check                                            |
