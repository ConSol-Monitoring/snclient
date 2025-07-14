---
title: service (linux)
---

## check_service

Checks the state of one or multiple linux (systemctl) services.

There is a specific [check_service for windows](../check_service_windows) as well.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows | Linux              | FreeBSD | MacOSX |
|:-------:|:------------------:|:-------:|:------:|
|         | :white_check_mark: |         |        |

## Examples

### Default Check

Checking all services except some excluded ones:

    check_service exclude=bluetooth
    OK - All 74 service(s) are ok.

Or check a specific service and get some metrics:

    check_service service=docker ok-syntax='${top-syntax}' top-syntax='%(status) - %(list)' detail-syntax='%(name) %(state) - memory: %(rss:h)B - age: %(age:duration)'
    OK - docker running - memory: 805.2MB - created: Fri 2023-11-17 20:34:01 CET |'docker'=4 'docker rss'=805200000B

Check memory usage of specific service:

    check_service service=docker warn='rss > 1GB' warn='rss > 2GB'
    OK - All 1 service(s) are ok. |'docker'=4 'docker rss'=59691008B;;;0 'docker vms'=3166244864B;;;0 'docker cpu'=0.7%;;;0 'docker tasks'=20;;;0

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

| Argument      | Default Value                                                                                     |
| ------------- | ------------------------------------------------------------------------------------------------- |
| filter        | active != inactive                                                                                |
| critical      | ( state not in ('running', 'oneshot', 'static') \|\| active = 'failed' )  && preset != 'disabled' |
| empty-state   | 3 (UNKNOWN)                                                                                       |
| empty-syntax  | %(status) - No services found                                                                     |
| top-syntax    | %(status) - %(crit_list)                                                                          |
| ok-syntax     | %(status) - All %(count) service(s) are ok.                                                       |
| detail-syntax | \${name}=\${state}                                                                                |

## Check Specific Arguments

| Argument | Description                                                                                           |
| -------- | ----------------------------------------------------------------------------------------------------- |
| exclude  | List of services to exclude from the check (mainly used when service is set to \*) (case insensitive) |
| service  | List of services to check (set to \* to check all services). (case insensitive) Default: \*           |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Description                                                                              |
| --------- | ---------------------------------------------------------------------------------------- |
| name      | The name of the service                                                                  |
| service   | Alias for name                                                                           |
| desc      | Description of the service                                                               |
| active    | The active attribute of a service, one of: active, inactive or failed                    |
| state     | The state of the service, one of: stopped, starting, oneshot, running, static or unknown |
| pid       | The pid of the service                                                                   |
| created   | Date when service was started                                                            |
| age       | Seconds since service was started                                                        |
| rss       | Memory rss in bytes (main process)                                                       |
| vms       | Memory vms in bytes (main process)                                                       |
| cpu       | CPU usage in percent (main process)                                                      |
| preset    | The preset attribute of the service, one of: enabled or disabled                         |
| tasks     | Number of tasks for this service                                                         |
