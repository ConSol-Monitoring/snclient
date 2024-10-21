---
title: ping
---

## check_ping

Checks the icmp ping connection.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_ping host=localhost
    OK - Packet loss = 0%, RTA = 0.113ms |...

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_ping
        use                  generic-service
        check_command        check_nrpe!check_ping!'warn=rta > 1000 || pl > 30' 'crit=rta > 5000 || pl > 80'
    }

## Argument Defaults

| Argument      | Default Value                                                     |
| ------------- | ----------------------------------------------------------------- |
| filter        | none                                                              |
| warning       | rta > 1000 \|\| pl > 30                                           |
| critical      | rta > 5000 \|\| pl > 80                                           |
| empty-state   | 3 (UNKNOWN)                                                       |
| empty-syntax  | %(status) - could not get any ping data                           |
| top-syntax    | %(status) - \${list}                                              |
| ok-syntax     |                                                                   |
| detail-syntax | Packet loss = \${pl}%{{ IF rta != '' }}, RTA = \${rta}ms{{ END }} |

## Check Specific Arguments

| Argument | Description                                      |
| -------- | ------------------------------------------------ |
| -4       | Force using IPv4.                                |
| -6       | Force using IPv6.                                |
| host     | host name or ip address to ping                  |
| packets  | number of ICMP ECHO packets to send (default: 5) |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Description                 |
| --------- | --------------------------- |
| host_name | host name ping was sent to. |
| ttl       | time to live.               |
| sent      | number of packets sent.     |
| received  | number of packets received. |
| rta       | average round trip time.    |
| pl        | packet loss in percent.     |
