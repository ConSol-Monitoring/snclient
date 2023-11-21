---
title: drivesize
---

## check_drivesize

Checks the disk drive/volumes usage on a host.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_drivesize drive=/
    OK: All 1 drive(s) are ok |'/ used'=296820846592B;;;0;489570443264 '/ used %'=60.6%;;;0;100

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_drivesize
        use                  generic-service
        check_command        check_nrpe!check_drivesize!'warn=used_pct > 90' 'crit=used_pct > 95'
    }

## Argument Defaults

| Argument      | Default Value                                                                                                                                                                                                         |
| ------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| filter        | fstype not in ('binfmt_misc', 'bpf', 'cgroup2fs', 'configfs', 'debugfs', 'devpts', 'efivarfs', 'fusectl', 'hugetlbfs', 'mqueue', 'nfsd', 'proc', 'pstorefs', 'ramfs', 'rpc_pipefs', 'securityfs', 'sysfs', 'tracefs') |
| warning       | used_pct > 80                                                                                                                                                                                                         |
| critcal       | used_pct > 90                                                                                                                                                                                                         |
| empty-state   | 3 (UNKNOWN)                                                                                                                                                                                                           |
| empty-syntax  | %(status): No drives found                                                                                                                                                                                            |
| top-syntax    | \${status}: \${problem_list}                                                                                                                                                                                          |
| ok-syntax     | %(status): All %(count) drive(s) are ok                                                                                                                                                                               |
| detail-syntax | %(drive_or_name) %(used)/%(size) used                                                                                                                                                                                 |

## Check Specific Arguments

| Argument          | Description                                                                                                              |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------ |
| drive             | The drives to check                                                                                                      |
| exclude           | List of drives to exclude from check                                                                                     |
| ignore-unreadable | Deprecated, use filter instead                                                                                           |
| magic             | Magic number for use with scaling drive sizes. Note there is also a more generic magic factor in the perf-config option. |
| mounted           | Deprecated, use filter instead                                                                                           |
| total             | Include the total of all matching drives                                                                                 |

## Attributes

### Check Specific Attributes

these can be used in filters and thresholds (along with the default attributes):

| Attribute       | Description                             |
| --------------- | --------------------------------------- |
| drive           | Technical name of drive                 |
| name            | Descriptive name of drive               |
| id              | Drive or id of drive                    |
| drive_or_id     | Drive letter if present if not use id   |
| drive_or_name   | Drive letter if present if not use name |
| fstype          | Filesystem type                         |
| free            | Free (human readable) bytes             |
| free_bytes      | Number of free bytes                    |
| free_pct        | Free bytes in percent                   |
| inodes_free     | Number of free inodes                   |
| inodes_free_pct | Number of free inodes in percent        |
| inodes_total    | Number of total free inodes             |
| inodes_used     | Number of used inodes                   |
| inodes_used_pct | Number of used inodes in percent        |
| mounted         | Flag wether drive is mounter (0/1)      |
| size            | Total size in human readable bytes      |
| size_bytes      | Total size in bytes                     |
| used            | Used (human readable) bytes             |
| used_bytes      | Number of used bytes                    |
| used_pct        | Used bytes in percent                   |
