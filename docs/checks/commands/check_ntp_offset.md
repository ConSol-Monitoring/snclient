---
title: ntp_offset
---

## check_ntp_offset

Checks the ntp offset.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_ntp_offset
    OK - offset 2.1ms from 1.2.3.4 (debian.pool.ntp.org) |...

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_ntp_offset
        use                  generic-service
        check_command        check_nrpe!check_ntp_offset!'warn=offset > 50 || offset < -50' 'crit=offset > 100 || offset < -100'
    }

## Argument Defaults

| Argument      | Default Value                                      |
| ------------- | -------------------------------------------------- |
| filter        | none                                               |
| warning       | offset > 50 \|\| offset < -50                      |
| critical      | offset > 100 \|\| offset < -100                    |
| empty-state   | 3 (UNKNOWN)                                        |
| empty-syntax  | %(status) - could not get any ntp data             |
| top-syntax    | %(status) - \${list}                               |
| ok-syntax     |                                                    |
| detail-syntax | offset \${offset_seconds:duration} from \${server} |

## Check Specific Arguments

| Argument | Description                                                                                                |
| -------- | ---------------------------------------------------------------------------------------------------------- |
| server   | Fetch offset from this ntp server(s). First valid response is used.                                        |
| source   | Set source of time data instead of auto detect. Valid values are: auto, timedatectl, ntpq, chronyc, osx, w32tm. |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute      | Description                                                                                          |
| -------------- | ---------------------------------------------------------------------------------------------------- |
| source         | source of the ntp metrics                                                                            |
| server         | ntp server name                                                                                      |
| stratum        | stratum value (distance to root ntp server)                                                          |
| jitter         | jitter of the clock in milliseconds                                                                  |
| offset         | time offset to ntp server in milliseconds. This will be added as a metric.                           |
| offset_seconds | time offset to ntp server in seconds. This will not be added as a metric.  Any thresholds using 'offset_seconds' will be converted to 'offset' silently. |
