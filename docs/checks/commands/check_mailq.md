---
title: mailq
---

## check_mailq

Checks the mailq.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows | Linux              | FreeBSD            | MacOSX             |
|:-------:|:------------------:|:------------------:|:------------------:|
|         | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_mailq
    OK: postfix: active 0 / deferred 0 |...

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_mailq
        use                  generic-service
        check_command        check_nrpe!check_mailq!warn='active > 5 || deferred > 0' crit='active > 10 || deferred > 10'
    }

## Argument Defaults

| Argument      | Default Value                                                                    |
| ------------- | -------------------------------------------------------------------------------- |
| filter        | none                                                                             |
| warning       | active > 5 \|\| active_size > 10MB \|\| deferred > 0 \|\| deferred_size > 10MB   |
| critical      | active > 10 \|\| active_size > 20MB \|\| deferred > 10 \|\| deferred_size > 20MB |
| empty-state   | 3 (UNKNOWN)                                                                      |
| empty-syntax  | \${status}: could not get any mailq data                                         |
| top-syntax    | \${status}: \${list}                                                             |
| ok-syntax     |                                                                                  |
| detail-syntax | \${mta}: active \${active} / deferred \${deferred}                               |

## Check Specific Arguments

| Argument | Description                                                                      |
| -------- | -------------------------------------------------------------------------------- |
| mta      | Set source mta for checking mailq instead of auto detect. Can be postfix or auto |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute     | Description                     |
| ------------- | ------------------------------- |
| mta           | name of the mta                 |
| folder        | checked spool folder            |
| active        | number of active mails          |
| active_size   | size of active mails in bytes   |
| deferred      | number of deferred mails        |
| deferred_size | size of deferred mails in bytes |
