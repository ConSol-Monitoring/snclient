---
title: memory
---

## check_memory

Checks the memory usage on the host.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_memory
    OK: physical = 6.98 GiB, committed = 719.32 MiB|...

Changing the return syntax to get more information:

    check_memory 'top-syntax=${list}' 'detail-syntax=${type} free: ${free} used: ${used} size: ${size}'
    physical free: 35.00 B used: 7.01 GiB size: 31.09 GiB, committed free: 27.00 B used: 705.57 MiB size: 977.00 MiB |...

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name    check_nrpe
        command_line    $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name               testhost
        service_description     check_memory
        use                     generic-service
        check_command           check_nrpe!check_memory!'warn=used > 80%' 'crit=used > 90%'
    }

## Argument Defaults

| Argument      | Default Value        |
| ------------- | -------------------- |
| warning       | used > 80%           |
| critcal       | used > 90%           |
| empty-state   | 0 (OK)               |
| empty-syntax  |                      |
| top-syntax    | \${status}: \${list} |
| ok-syntax     |                      |
| detail-syntax | %(type) = %(used)    |

## Check Specific Arguments

| Argument | Description                                          |
| -------- | ---------------------------------------------------- |
| type     | Type of memory to check. Default: physical,committed |

## Attributes

### Check Specific Attributes

these can be used in filters and thresholds (along with the default attributes):

| Attribute  | Description                                           |
| ---------- | ----------------------------------------------------- |
| <type>     | used bytes with the type as key                       |
| type       | checked type, either 'physical' or 'committed' (swap) |
| used       | Used memory in human readable bytes (IEC)             |
| used_bytes | Used memory in bytes (IEC)                            |
| used_pct   | Used memory in percent                                |
| free       | Free memory in human readable bytes (IEC)             |
| free_bytes | Free memory in bytes (IEC)                            |
| free_pct   | Free memory in percent                                |
| size       | Total memory in human readable bytes (IEC)            |
| size_bytes | Total memory in bytes (IEC)                           |
