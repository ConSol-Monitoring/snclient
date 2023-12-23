---
title: network
---

## check_network

Checks the state and metrics of network interfaces.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_network device=eth0
    OK: eth0 >12 kB/s <28 kB/s |...

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_network
        use                  generic-service
        check_command        check_nrpe!check_network!
    }

## Argument Defaults

| Argument      | Default Value                 |
| ------------- | ----------------------------- |
| warning       | total > 10000                 |
| critical      | total > 100000                |
| empty-state   | 3 (UNKNOWN)                   |
| empty-syntax  | %(status): No devices found   |
| top-syntax    | %(status): %(list)            |
| ok-syntax     | %(status): %(list)            |
| detail-syntax | %(name) >%(sent) <%(received) |

## Check Specific Arguments

| Argument | Description                         |
| -------- | ----------------------------------- |
| dev      | Alias for device                    |
| device   | The device to check. Default is all |
| exclude  | Exclude device by name              |
| name     | Alias for device                    |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute         | Description                                              |
| ----------------- | -------------------------------------------------------- |
| MAC               | The MAC address                                          |
| enabled           | True if the network interface is enabled (true/false)    |
| name              | Name of the interface                                    |
| net_connection_id | same as name                                             |
| received          | Bytes received per second (calculated over the last 30s) |
| total_received    | Total bytes received                                     |
| sent              | Bytes sent per second (calculated over the last 30s)     |
| total_sent        | Total bytes sent                                         |
| speed             | Network interface speed                                  |
| flags             | Interface flags                                          |
| total             | Sum of sent and received bytes per second                |
