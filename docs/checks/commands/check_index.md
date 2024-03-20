---
title: index
---

## check_index

returns list of known checks.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_index filter="implemented = 1"
    check_cpu...

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_index
        use                  generic-service
        check_command        check_nrpe!check_index!
    }

## Argument Defaults

| Argument      | Default Value   |
| ------------- | --------------- |
| filter        | implemented = 1 |
| empty-state   | 3 (UNKNOWN)     |
| empty-syntax  | no checks found |
| top-syntax    | \${list}        |
| ok-syntax     |                 |
| detail-syntax | \${name}        |

## Check Specific Arguments

None

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute   | Description                                   |
| ----------- | --------------------------------------------- |
| name        | name of the check                             |
| description | description of the check                      |
| implemented | check is available on current platform: 0 / 1 |
| windows     | check is available on windows: 0 / 1          |
| linux       | check is available on linux: 0 / 1            |
| osx         | check is available on mac osx: 0 / 1          |
| freebsd     | check is available on freebsd: 0 / 1          |
| alias       | check is an alias: 0 / 1                      |
| script      | check is a (wrapped) script: 0 / 1            |
