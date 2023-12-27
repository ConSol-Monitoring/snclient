---
title: os_version
---

## check_os_version

Checks the os system version.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_os_version
    OK - Microsoft Windows 10 Pro 10.0.19045.2728 Build 19045.2728 (arch: amd64)

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_os_version
        use                  generic-service
        check_command        check_nrpe!check_os_version!
    }

## Argument Defaults

| Argument      | Default Value                             |
| ------------- | ----------------------------------------- |
| empty-state   | 0 (OK)                                    |
| empty-syntax  |                                           |
| top-syntax    | %(status) - \${list})                     |
| ok-syntax     |                                           |
| detail-syntax | \${platform} \${version} (arch: \${arch}) |

## Check Specific Arguments

None

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Description         |
| --------- | ------------------- |
| platform  | Platform of the OS  |
| family    | OS Family           |
| version   | Full version number |
| arch      | OS architecture     |
