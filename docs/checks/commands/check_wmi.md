---
title: wmi
---

## check_wmi

Check status and metrics by running wmi queries.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)

## Implementation

| Windows            | Linux | FreeBSD | MacOSX |
|:------------------:|:-----:|:-------:|:------:|
| :white_check_mark: |       |         |        |

## Examples

### Default Check

    check_wmi "query=select FreeSpace, DeviceID FROM Win32_LogicalDisk WHERE DeviceID = 'C:'"
    27955118080, C:

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_wmi
        use                  generic-service
        check_command        check_nrpe!check_wmi!
    }

## Argument Defaults

| Argument      | Default Value |
| ------------- | ------------- |
| empty-state   | 0 (OK)        |
| empty-syntax  |               |
| top-syntax    | \${list}      |
| ok-syntax     |               |
| detail-syntax | %(line)       |

## Check Specific Arguments

| Argument  | Description                      |
| --------- | -------------------------------- |
| namespace | Unused and not supported for now |
| password  | Unused and not supported for now |
| query     | The WMI query to execute         |
| target    | Unused and not supported for now |
| user      | Unused and not supported for now |
