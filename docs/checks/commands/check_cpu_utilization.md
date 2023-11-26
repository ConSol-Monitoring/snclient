---
title: cpu_utilization
---

## check_cpu_utilization

Checks the cpu utilization metrics.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_cpu_utilization
    OK - user: 29% - system: 11% - iowait: 3% - steal: 0% - guest: 0% |'user'=28.83%;;;0;...

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_cpu_utilization
        use                  generic-service
        check_command        check_nrpe!check_cpu_utilization!'warn=total > 90%' 'crit=total > 95%'
    }

## Argument Defaults

| Argument      | Default Value                                                                                       |
| ------------- | --------------------------------------------------------------------------------------------------- |
| warning       | total > 90                                                                                          |
| critcal       | total > 95                                                                                          |
| empty-state   | 0 (OK)                                                                                              |
| empty-syntax  |                                                                                                     |
| top-syntax    | \${status} - \${list}                                                                               |
| ok-syntax     |                                                                                                     |
| detail-syntax | user: \${user}% - system: \${system}% - iowait: \${iowait}% - steal: \${steal}% - guest: \${guest}% |

## Check Specific Arguments

| Argument | Description                                          |
| -------- | ---------------------------------------------------- |
| range    | Sets time range to calculate average (default is 1m) |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Description                                          |
| --------- | ---------------------------------------------------- |
| total     | Sum of user,system,iowait,steal and guest in percent |
| user      | User cpu utilization in percent                      |
| system    | System cpu utilization in percent                    |
| iowait    | IOWait cpu utilization in percent                    |
| steal     | Steal cpu utilization in percent                     |
| guest     | Guest cpu utilization in percent                     |
