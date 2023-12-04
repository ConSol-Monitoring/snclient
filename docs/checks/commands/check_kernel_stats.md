---
title: kernel_stats
---

## check_kernel_stats

Checks the metrics of the linux kernel.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows | Linux              | FreeBSD | MacOSX |
|:-------:|:------------------:|:-------:|:------:|
|         | :white_check_mark: |         |        |

## Examples

### Default Check

    check_kernel_stats
    OK: Context Switches 29.2/s, Process Creations 12.7/s |'ctxt'=29.2/s 'processes'=12.7/s

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_kernel_stats
        use                  generic-service
        check_command        check_nrpe!check_kernel_stats!
    }

## Argument Defaults

| Argument      | Default Value               |
| ------------- | --------------------------- |
| empty-state   | 3 (UNKNOWN)                 |
| empty-syntax  | %(status): No metrics found |
| top-syntax    | %(status): %(list)          |
| ok-syntax     | %(status): %(list)          |
| detail-syntax | %(label) %(rate:fmt=%.1f)/s |

## Check Specific Arguments

| Argument | Description                                           |
| -------- | ----------------------------------------------------- |
| type     | Select metric type to show, can be: ctxt or processes |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Description         |
| --------- | ------------------- |
| name      | Name of the metric  |
| label     | Label of the metric |
| rate      | Rate of this metric |
| current   | Current raw value   |
