---
title: swap_io
---

## check_swap_io

Checks the swap Input / Output rate on the host.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows | Linux              | FreeBSD            | MacOSX             |
|:-------:|:------------------:|:------------------:|:------------------:|
|         | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

	check_swap_io
	OK - 1 swaps >22.98 KiB/s <18.58 MiB/s |'swap_in'=54024851456c;;;0; 'swap_out'=179375353856c;;;0;

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_swap_io
        use                  generic-service
        check_command        check_nrpe!check_swap_io!
    }

## Argument Defaults

| Argument      | Default Value                                                       |
| ------------- | ------------------------------------------------------------------- |
| empty-state   | 0 (OK)                                                              |
| empty-syntax  |                                                                     |
| top-syntax    | %(status) - %(list)                                                 |
| ok-syntax     | %(status) - %(list)                                                 |
| detail-syntax | %(swap_count) swap device(s) >%(swap_in_rate)/s <%(swap_out_rate)/s |

## Check Specific Arguments

| Argument | Description                                                                           |
| -------- | ------------------------------------------------------------------------------------- |
| lookback | Lookback period for the value change rate calculations, given in seconds. Default: 60 |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute           | Description                 |
| ------------------- | --------------------------- |
| swap_count          | Count of swap partitions    |
| swap_in_rate_bytes  | Swap/Pages being brought in |
| swap_in_rate        | Swap/Pages being sent out   |
| swap_out_rate_bytes | Swap/Pages being brought in |
| swap_out_rate       | Swap/Pages being sent out   |
