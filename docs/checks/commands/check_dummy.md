---
title: dummy
---

## check_dummy

This check simply sets the state to the given value and outputs the remaining arguments.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_dummy 0 some example output
    some example output

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_dummy
        use                  generic-service
        check_command        check_nrpe!check_dummy!0 'some example output'
    }

## Check Specific Arguments

None
