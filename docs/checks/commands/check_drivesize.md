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

    check_drivesize drive=/ show-all
    OK - / 280.155 GiB/455.948 GiB (64.7%) |...

Check drive including inodes:

    check_drivesize drive=/ warn="used > 90%" "crit=used > 95%" "warn=inodes > 90%" "crit=inodes > 95%"
    OK - All 1 drive(s) are ok |'/ used'=307515822080B;440613398938;465091921101;0;489570443264 '/ used %'=62.8%;90;95;0;100 '/ inodes'=12.1%;90;95;0;100

Check folder, no matter if its a mountpoint itself or not:

    check_drivesize folder=/tmp show-all
    OK - /tmp 280.155 GiB/455.948 GiB (64.7%) |...

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

| Argument      | Default Value                                                                                         |
| ------------- | ----------------------------------------------------------------------------------------------------- |
| filter        | fstype not in ('autofs', 'bdev', 'binfmt_misc', 'bpf', 'cgroup', 'cgroup2', 'configfs', 'cpuset', 'debugfs', 'devpts', 'devtmpfs', 'efivarfs', 'fuse.portal', 'fusectl', 'hugetlbfs', 'mqueue', 'nsfs', 'overlay', 'pipefs', 'proc', 'pstore', 'ramfs', 'rpc_pipefs', 'securityfs', 'selinuxfs', 'sockfs', 'sysfs', 'tracefs') |
| warning       | used_pct > 80                                                                                         |
| critical      | used_pct > 90                                                                                         |
| empty-state   | 3 (UNKNOWN)                                                                                           |
| empty-syntax  | %(status) - No drives found                                                                           |
| top-syntax    | %(status) - \${problem_list}                                                                          |
| ok-syntax     | %(status) - All %(count) drive(s) are ok                                                              |
| detail-syntax | %(drive_or_name) %(used)/%(size) (%(used_pct \| fmt=%.1f )%)                                          |

## Check Specific Arguments

| Argument                  | Description                                                                               |
| ------------------------- | ----------------------------------------------------------------------------------------- |
| drive                     | The drives to check, ex.: c: or /                                                         |
| exclude                   | List of drives to exclude from check                                                      |
| folder                    | The folders to check (parent mountpoint)                                                  |
| freespace-ignore-reserved | Don't account root-reserved blocks into freespace, default: true                          |
| ignore-unreadable         | Deprecated, use filter instead                                                            |
| magic                     | Magic number for use with scaling drive sizes. Note there is also a more generic magic factor in the perf-config option. |
| mounted                   | Deprecated, use filter instead                                                            |
| total                     | Include the total of all matching drives                                                  |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute       | Description                                                                         |
| --------------- | ----------------------------------------------------------------------------------- |
| drive           | Technical name of drive                                                             |
| name            | Descriptive name of drive                                                           |
| id              | Drive or id of drive                                                                |
| drive_or_id     | Drive letter if present if not use id                                               |
| drive_or_name   | Drive letter if present if not use name                                             |
| fstype          | Filesystem type                                                                     |
| mounted         | Flag wether drive is mounter (0/1)                                                  |
| free            | Free (human readable) bytes                                                         |
| free_bytes      | Number of free bytes                                                                |
| free_pct        | Free bytes in percent                                                               |
| user_free       | Number of total free bytes (from user perspective)                                  |
| user_free_pct   | Number of total % free space (from user perspective)                                |
| total_free      | Number of total free bytes                                                          |
| total_free_pct  | Number of total % free space                                                        |
| used            | Used (human readable) bytes                                                         |
| used_bytes      | Number of used bytes                                                                |
| used_pct        | Used bytes in percent (from user perspective)                                       |
| user_used       | Number of total used bytes (from user perspective)                                  |
| user_used_pct   | Number of total % used space                                                        |
| total_used      | Number of total used bytes (including root reserved)                                |
| total_used_pct  | Number of total % used space  (including root reserved)                             |
| size            | Total size in human readable bytes                                                  |
| size_bytes      | Total size in bytes                                                                 |
| inodes_free     | Number of free inodes                                                               |
| inodes_free_pct | Number of free inodes in percent                                                    |
| inodes_total    | Number of total free inodes                                                         |
| inodes_used     | Number of used inodes                                                               |
| inodes_used_pct | Number of used inodes in percent                                                    |
| media_type      | Windows only: numeric media type of drive                                           |
| type            | Windows only: type of drive, ex.: fixed, cdrom, ramdisk, remote, removable, unknown |
| readable        | Windows only: flag drive is readable (0/1)                                          |
| writable        | Windows only: flag drive is writable (0/1)                                          |
| removable       | Windows only: flag drive is removable (0/1)                                         |
| erasable        | Windows only: flag wether if drive is erasable (0/1)                                |
| hotplug         | Windows only: flag drive is hotplugable (0/1)                                       |
