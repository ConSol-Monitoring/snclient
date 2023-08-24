---
title: drivesize
---

# check_drivesize

Check the size (free-space) of a drive or volume.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

### Implementation

| Windows | Linux | FreeBSD | MacOSX |
|:-------:|:-----:|:-------:|:------:|
| :white_check_mark: | :construction: | :construction: | :construction: |

## Examples

### Default check

    check_drivesize drive=c:
    OK: All 1 drive(s) are ok


### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name    check_nrpe
        command_line    $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
            host_name               testhost
            service_description     check_drivesize_testhost
            check_command           check_nrpe!check_drivesize!'drive=*' 'warn=used > 80' 'crit=used > 95'
    }

Return

    OK: All 1 drive(s) are ok

## Argument Defaults

| Argument | Default Value |
| --- | --- |
filter | ( mounted = 1  or media_type = 0 ) |
warning | used > 80 |
critical | used > 90 |
top-syntax | \${status} ${problem_list} |
ok-syntax | %(status) All %(count) drive(s) are ok |
empty-syntax | %(status): No drives found |
detail-syntax | %(drive_or_name) %(used)/%(size) used |

### Check specific arguments

| Argument | Description |
| --- | --- |
| drive | The drives to check |
| magic | Magic number for use with scaling drive sizes. Note there is also a more generic magic factor in the perf-config option. |
| exclude | List of drives to exclude from check |
| total | Include the total of all matching drives |

## Attributes

### Check specific attributes

| Attribute | Description |
| --- | --- |
| id | Drive or id of drive |
| name | Descriptive name of drive |
| drive | Name of the drive |
| drive_or_id | Drive letter if present if not use id |
| drive_or_name | Drive letter if present if not use name |
| media_type | The media type |
| type | Type of drive |
| letter | Letter the drive is mounted on |
| size | Total size of drive (human readable) |
| size_bytes | Total size of drive in bytes |
| used | Total used of drive (human readable) |
| used_bytes | Total used of drive in bytes |
| used_pct | Total used of drive in percent |
| free | Total free of drive (human readable) |
| free_bytes | Total free of drive in bytes |
| free_pct | Total free of drive in percent |

### Notes

If no unit is specified, `used` and `free` default to percent, so `warning=used > 15` and `warning=used > 15%` is the same.