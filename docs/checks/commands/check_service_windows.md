---
title: service (windows)
---

## check_service

Checks the state of one or multiple windows services.

There is a specific [check_service for linux](../check_service_linux) as well.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux | FreeBSD | MacOSX |
|:------------------:|:-----:|:-------:|:------:|
| :white_check_mark: |       |         |        |

## Examples

### Default Check

Checking all services except some excluded ones:

    check_service exclude=edgeupdate exclude=RemoteRegistry
    OK - All 15 service(s) are ok |'count'=15;;;0 'failed'=0;;;0

Checking a single service:

    check_service service=dhcp
    OK - All 1 service(s) are ok.

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
        check_command        check_nrpe!check_service!service=dhcp
    }

## Argument Defaults

| Argument      | Default Value                                    |
| ------------- | ------------------------------------------------ |
| filter        | none                                             |
| warning       | state != 'running' && start_type = 'delayed'     |
| critical      | state != 'running' && start_type = 'auto'        |
| empty-state   | 3 (UNKNOWN)                                      |
| empty-syntax  | %(status) - No services found                    |
| top-syntax    | %(status) - %(crit_list), delayed (%(warn_list)) |
| ok-syntax     | %(status) - All %(count) service(s) are ok.      |
| detail-syntax | \${name}=\${state} (\${start_type})              |

## Check Specific Arguments

| Argument | Description                                                                                           |
| -------- | ----------------------------------------------------------------------------------------------------- |
| exclude  | List of services to exclude from the check (mainly used when service is set to \*) (case insensitive) |
| service  | Name of the service to check (set to \* to check all services). (case insensitive) Default: \*        |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute      | Description                                                                                          |
| -------------- | ---------------------------------------------------------------------------------------------------- |
| name           | The name of the service                                                                              |
| service        | Alias for name                                                                                       |
| desc           | Description of the service                                                                           |
| state          | The state of the service, one of: stopped, starting, stopping, running, continuing, pausing, paused or unknown |
| pid            | The pid of the service                                                                               |
| created        | Date when service was started                                                                        |
| age            | Seconds since service was started                                                                    |
| rss            | Memory rss in bytes                                                                                  |
| vms            | Memory vms in bytes                                                                                  |
| cpu            | CPU usage in percent                                                                                 |
| delayed        | If the service is delayed, can be 0 or 1                                                             |
| classification | Classification of the service, one of: kernel-driver, system-driver, service-adapter, driver, service-own-process, service-shared-process, service or interactive |
| start_type     | The configured start type, one of: boot, system, delayed, auto, demand, disabled or unknown          |
