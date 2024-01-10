---
title: pagefile
---

## check_pagefile

Checks the pagefile usage.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux | FreeBSD | MacOSX |
|:------------------:|:-----:|:-------:|:------:|
| :white_check_mark: |       |         |        |

## Examples

### Default Check

    check_pagefile
    OK - total 39.10 MiB/671.39 MiB (5.8%) |...

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_pagefile
        use                  generic-service
        check_command        check_nrpe!check_pagefile!'warn=used > 80%' 'crit=used > 95%'
    }

## Argument Defaults

| Argument      | Default Value                                          |
| ------------- | ------------------------------------------------------ |
| filter        | name = 'total'                                         |
| warning       | used > 60%                                             |
| critical      | used > 80%                                             |
| empty-state   | 0 (OK)                                                 |
| empty-syntax  |                                                        |
| top-syntax    | %(status) - \${list}                                   |
| ok-syntax     |                                                        |
| detail-syntax | \${name} \${used}/\${size} (%(used_pct \| fmt=%.1f )%) |

## Check Specific Arguments

None

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute  | Description                               |
| ---------- | ----------------------------------------- |
| name       | The name of the page file (location)      |
| used       | Used memory in human readable bytes       |
| used_bytes | Used memory in bytes                      |
| used_pct   | Used memory in percent                    |
| free       | Free memory in human readable bytes       |
| free_bytes | Free memory in bytes                      |
| free_pct   | Free memory in percent                    |
| peak       | Peak memory usage in human readable bytes |
| peak_bytes | Peak memory in bytes                      |
| peak_pct   | Peak memory in percent                    |
| size       | Total size of pagefile (human readable)   |
| size_bytes | Total size of pagefile in bytes           |
