---
title: service (linux)
---

## check_service

Checks the state of one or multiple linux (systemctl) services.

There is a specific [check_service for windows](check_service_windows) as well.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows | Linux              | FreeBSD | MacOSX |
|:-------:|:------------------:|:-------:|:------:|
|         | :white_check_mark: |         |        |

## Examples

### Default Check

    check_service
    OK: All 74 service(s) are ok.

Or check a specific service and get some metrics:

    check_service service=docker ok-syntax='%(status): %(list)' detail-syntax='%(name) - memory: %(mem) - created: %(created)'
    OK: docker - memory: 805.2M - created: Fri 2023-11-17 20:34:01 CET |'docker'=4 'docker mem'=805200000B

Check memory usage of specific service:

    check_service service=docker warn='mem > 1GB' warn='mem > 2GB'
    OK: All 1 service(s) are ok. |'docker'=4 'docker mem'=793700000B;1000000000;2000000000;0

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_service
        use                  generic-service
        check_command        check_nrpe!check_service!service=docker
    }

## Argument Defaults

| Argument      | Default Value                                                         |
| ------------- | --------------------------------------------------------------------- |
| filter        | none                                                                  |
| critcal       | state not in ('running', 'oneshot', 'static') && preset != 'disabled' |
| empty-state   | 3 (UNKNOWN)                                                           |
| empty-syntax  | %(status): No services found                                          |
| top-syntax    | %(status): %(crit_list)                                               |
| ok-syntax     | %(status): All %(count) service(s) are ok.                            |
| detail-syntax | \${name}=\${state}                                                    |

## Check Specific Arguments

| Argument | Description                                                                        |
| -------- | ---------------------------------------------------------------------------------- |
| exclude  | List of services to exclude from the check (mainly used when service is set to \*) |
| service  | Name of the service to check (set to \* to check all services). Default: \*        |

## Attributes

### Check Specific Attributes

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Description                                                                      |
| --------- | -------------------------------------------------------------------------------- |
| name      | The name of the service                                                          |
| service   | Alias for name                                                                   |
| desc      | Description of the service                                                       |
| state     | The state of the service, one of: stopped, starting, oneshot, running or unknown |
| created   | Date when service was created                                                    |
| preset    | The preset attribute of the service, one of: enabled or disabled                 |
| pid       | The pid of the service                                                           |
| mem       | The memory usage in human readable bytes                                         |
| mem_bytes | The memory usage in bytes                                                        |
