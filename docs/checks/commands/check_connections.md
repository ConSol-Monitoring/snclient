---
title: connections
---

## check_connections

Checks the number of tcp connections.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD | MacOSX             |
|:------------------:|:------------------:|:-------:|:------------------:|
| :white_check_mark: | :white_check_mark: |         | :white_check_mark: |

## Examples

### Default Check

    check_connections
    OK: total connections 60

Check only ipv6 connections:

    check_connections inet=ipv6
    OK: total ipv6 connections 13

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_connections
        use                  generic-service
        check_command        check_nrpe!check_connections!'warn=total > 500' 'crit=total > 1500'
    }

## Argument Defaults

| Argument      | Default Value                          |
| ------------- | -------------------------------------- |
| filter        | inet=total                             |
| warning       | total > 1000                           |
| critical      | total > 2000                           |
| empty-state   | 0 (OK)                                 |
| empty-syntax  |                                        |
| top-syntax    | \${status}: \${list}                   |
| ok-syntax     |                                        |
| detail-syntax | total \${prefix}connections: \${total} |

## Check Specific Arguments

| Argument | Description                                                        |
| -------- | ------------------------------------------------------------------ |
| inet     | Use specific address family only. Can be: total, any, ipv4 or ipv6 |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute    | Description                                                                             |
| ------------ | --------------------------------------------------------------------------------------- |
| inet         | address family, can be total (sum of any), all (any+total), any (v4+v6), inet4 or inet6 |
| prefix       | address family as prefix, will be empty, inet4 or inet6                                 |
| total        | total number of connections                                                             |
| established  | total number of connections of type: established                                        |
| syn_sent     | total number of connections of type: syn_sent                                           |
| syn_recv     | total number of connections of type: syn_recv                                           |
| fin_wait1    | total number of connections of type: fin_wait1                                          |
| fin_wait2    | total number of connections of type: fin_wait2                                          |
| time_wait    | total number of connections of type: time_wait                                          |
| close        | total number of connections of type: close                                              |
| close_wait   | total number of connections of type: close_wait                                         |
| last_ack     | total number of connections of type: last_ack                                           |
| listen       | total number of connections of type: listen                                             |
| closing      | total number of connections of type: closing                                            |
| new_syn_recv | total number of connections of type: new_syn_recv                                       |
