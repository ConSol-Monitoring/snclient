---
title: os_updates
---

## check_os_updates

Checks for OS system updates.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_os_updates
    OK - no updates available

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_os_updates
        use                  generic-service
        check_command        check_nrpe!check_os_updates!warn='count > 0' crit='count_security > 0'
    }

## Argument Defaults

| Argument      | Default Value                                                                |
| ------------- | ---------------------------------------------------------------------------- |
| warning       | count > 0                                                                    |
| critical      | count_security > 0                                                           |
| empty-state   | 0 (OK)                                                                       |
| empty-syntax  | %(status) - no updates available                                             |
| top-syntax    | %(status) - %{count_security} security updates / %{count} updates available. |
| ok-syntax     |                                                                              |
| detail-syntax | \${package}: \${version}
                                                    |

## Check Specific Arguments

| Argument | Description                                |
| -------- | ------------------------------------------ |
| update   | Update package list, (ex.: apt-get update) |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Description                      |
| --------- | -------------------------------- |
| package   | package name                     |
| security  | is this a security update: 0 / 1 |
| version   | version string of package        |
