---
title: load
---

## check_load

Checks the cpu load metrics.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_load
    OK - total load average: 2.36, 1.26, 1.01 |'load1'=2.36;;;0 'load5'=1.26;;;0 'load15'=1.01;;;0

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_load
        use                  generic-service
        check_command        check_nrpe!check_load!'warn=load > 20' 'crit=load > 30'
    }

## Argument Defaults

| Argument      | Default Value                                           |
| ------------- | ------------------------------------------------------- |
| filter        | none                                                    |
| empty-state   | 0 (OK)                                                  |
| empty-syntax  |                                                         |
| top-syntax    | %(status) - \${list} on \${cores} cores                 |
| ok-syntax     |                                                         |
| detail-syntax | \${type} load average: \${load1}, \${load5}, \${load15} |

## Check Specific Arguments

| Argument            | Description                                                           |
| ------------------- | --------------------------------------------------------------------- |
| --show-args         | Show arguments when listing the top N processes                       |
| -c\|--critical      | Critical threshold: CLOAD1,CLOAD5,CLOAD15                             |
| -n\|--procs-to-show | Number of processes to show when printing the top consuming processes |
| -r\|--percpu        | Divide the load averages by the number of CPUs                        |
| -w\|--warning       | Warning threshold: WLOAD1,WLOAD5,WLOAD15                              |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Description                              |
| --------- | ---------------------------------------- |
| type      | type will be either 'total' or 'scaled'  |
| load1     | average load value over 1 minute         |
| load5     | average load value over 5 minutes        |
| load15    | average load value over 15 minutes       |
| load      | maximum value of load1, load5 and load15 |
