---
title: uptime
---

## check_uptime

Check computer uptime (time since last reboot).

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_uptime
    OK - uptime: 3d 02:30h, boot: 2023-11-17 19:33:46 UTC |'uptime'=268241s;172800:;86400:

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_uptime
        use                  generic-service
        check_command        check_nrpe!check_uptime!'warn=uptime < 180s' 'crit=uptime < 60s'
    }

## Argument Defaults

| Argument      | Default Value                             |
| ------------- | ----------------------------------------- |
| warning       | uptime < 2d                               |
| critical      | uptime < 1d                               |
| empty-state   | 0 (OK)                                    |
| empty-syntax  |                                           |
| top-syntax    | %(status) - \${list}                      |
| ok-syntax     |                                           |
| detail-syntax | uptime: \${uptime}, boot: \${boot \| utc} |

## Check Specific Arguments

None

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute    | Description                         |
| ------------ | ----------------------------------- |
| uptime       | Human readable time since last boot |
| uptime_value | Uptime in seconds                   |
| boot         | Unix timestamp of last boot         |
