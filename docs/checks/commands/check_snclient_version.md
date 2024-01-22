---
title: snclient_version
---

## check_snclient_version

Check and return snclient version.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_snclient_version
    SNClient+ v0.12.0036 (Build: 5e351bb, go1.21.6)

There is an alias 'check_nscp_version' for this command.

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_snclient_version
        use                  generic-service
        check_command        check_nrpe!check_snclient_version!
    }

## Argument Defaults

| Argument      | Default Value                                   |
| ------------- | ----------------------------------------------- |
| empty-state   | 0 (OK)                                          |
| empty-syntax  |                                                 |
| top-syntax    | \${list}                                        |
| ok-syntax     |                                                 |
| detail-syntax | \${name} \${version} (Build: \${build}, \${go}) |

## Check Specific Arguments

None

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Description                 |
| --------- | --------------------------- |
| name      | The name of this agent      |
| version   | Version string              |
| build     | git commit id of this build |
