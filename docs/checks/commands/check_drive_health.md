---
title: drive_health
---

## check_drive_health

Runs a SMART test and reports the test result alongside the smart health status.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows | Linux              | FreeBSD | MacOSX |
|:-------:|:------------------:|:-------:|:------:|
|         | :white_check_mark: |         |        |

## Examples

### Default Check

Perform an offline test on all drives
		check_drive_health

Perform a short test on a specific NVMe drive
		check_drive_health test_type='short' drive_filter='/dev/nvme0'

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_drive_health
        use                  generic-service
        check_command        check_nrpe!check_drive_health!
    }

## Argument Defaults

| Argument      | Default Value                                                                                         |
| ------------- | ----------------------------------------------------------------------------------------------------- |
| warning       |  return_code != '0' \|\| test_result != 'PASSED' \|\| smart_health_test_result != 'PASSED'            |
| critical      |  return_code != '0' && test_result != 'PASSED' && smart_health_test_result != 'PASSED'                |
| empty-state   | 3 (UNKNOWN)                                                                                           |
| empty-syntax  | Failed to find any drives that the filter and smartctl could work with                                |
| top-syntax    | %(status) - %(problem_count)/%(count) drives , %(problem_list)                                        |
| ok-syntax     | %(status) - All %(count) drives are ok                                                                |
| detail-syntax | drive: %(test_drive) \| test: %(test_type) \| test_result: %(test_result) \| smart_health_test_result: %(smart_health_test_result) \| return_code: %(return_code) , (%(return_code_explanation)) |

## Check Specific Arguments

| Argument     | Description                                                                                            |
| ------------ | ------------------------------------------------------------------------------------------------------ |
| drive_filter | Drives to check health for. Give it as a comma separated list of logical device names e.g '/dev/sda,'/dev/nvme0' . Leaving it empty will check all drives which report a SMART status. |
| test_type    | SMART test type to perform for checking the health of the drives. Available test types are: 'offline,short,long'  |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute                | Description                                                                                |
| ------------------------ | ------------------------------------------------------------------------------------------ |
| test_type                | Type of SMART test that was performed.                                                     |
| test_drive               | The drive the test was performed on                                                        |
| test_result              | The result of the test. Takes possible outputs: "PASSED" , "FAILED" , "UNKNOWN" .          |
| test_details             | The details of the test given by smartctl.                                                 |
| smart_health_test_result | SMART overall health self-assesment done by the firmware with the current SMART attributes.  It is evaluated independently from the test result, but is just as important. Takes possible values: "PASSED" , "FAILED" , "UNKNOWN" . |
| return_code              | The return code status of the smartctl command used to get drive details after the test was done. |
| return_code_explanation  | Explanation of the return code of the smartctl command used to get drive details after the test was done. |
