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

    check_wmi "query=select DeviceID, FreeSpace FROM Win32_LogicalDisk WHERE DeviceID = 'C:'"
    C:, 27955118080

Same query, but use output template for custom plugin output:

    check_wmi "query=select DeviceID, FreeSpace FROM Win32_LogicalDisk WHERE DeviceID = 'C:'" "detail-syntax= %(DeviceID) %(FreeSpace:h)"
    C: 27.94 G

Performance data will be extracted if the query contains at least 2 attributes. The first one must be a name, the others must be numeric.

    check_wmi query="select DeviceID, FreeSpace, Size FROM Win32_LogicalDisk"
	C:, 27199328256 |'FreeSpace_C'=27199328256

Use perf-config to format the performance data and apply thresholds:

    check_wmi query="select DeviceID, FreeSpace FROM Win32_LogicalDisk" perf-config="*(unit:B)" warn="FreeSpace > 1000000"
	C:, 27199328256 |'FreeSpace_C'=27199328256;1000000

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

| Argument      | Default Value                        |
| ------------- | ------------------------------------ |
| empty-state   | 3 (UNKNOWN)                          |
| empty-syntax  | query did not return any result row. |
| top-syntax    | \${list}                             |
| ok-syntax     |                                      |
| detail-syntax | %(line)                              |

## Check Specific Arguments

| Argument  | Description                      |
| --------- | -------------------------------- |
| namespace | Unused and not supported for now |
| password  | Unused and not supported for now |
| query     | The WMI query to execute         |
| target    | Unused and not supported for now |
| user      | Unused and not supported for now |
