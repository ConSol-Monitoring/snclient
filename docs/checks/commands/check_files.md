---
title: files
---

## check_files

Checks files and directories.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_files 'path=/tmp' 'filter=age > 3d' 'warn=count > 500' 'crit=count > 600'
    OK: All 138 files are ok: (29.22 MiB) |'count'=138;500;600;0 'size'=30642669B;;;0

Check for folder size:

    check_files 'path=/tmp' 'warn=total_size > 200MiB' 'crit=total_size > 300MiB'
    OK: All 145 files are ok: (34.72 MiB) |'count'=145;;;0 'size'=36406741B;209715200;314572800;0

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_files
        use                  generic-service
        check_command        check_nrpe!check_files!'path=/tmp' 'filter=age > 3d' 'warn=count > 500' 'crit=count > 600'
    }

## Argument Defaults

| Argument      | Default Value                                                              |
| ------------- | -------------------------------------------------------------------------- |
| empty-state   | 3 (UNKNOWN)                                                                |
| empty-syntax  | No files found                                                             |
| top-syntax    | %(status): %(problem_count)/%(count) files (%(total_size)) %(problem_list) |
| ok-syntax     | %(status): All %(count) files are ok: (%(total_size))                      |
| detail-syntax | %(name)                                                                    |

## Check Specific Arguments

| Argument  | Description                                                |
| --------- | ---------------------------------------------------------- |
| file      | Alias for path                                             |
| max-depth | Maximum recursion depth                                    |
| path      | Path in which to search for files                          |
| paths     | A comma separated list of paths                            |
| pattern   | Pattern of files to search for                             |
| timezone  | Sets the timezone for time metrics (default is local time) |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute   | Description                                       |
| ----------- | ------------------------------------------------- |
| path        | Path of the file                                  |
| access      | Last access time                                  |
| age         | Seconds since file was last written               |
| creation    | Date when file was created                        |
| file        | Name of the file                                  |
| filename    | Name of the file                                  |
| name        | Name of the file                                  |
| fullname    | Full name of the file including path              |
| size        | File size in bytes                                |
| type        | Type of item (file or directory)                  |
| written     | Date when file was last written to                |
| write       | Alias for written                                 |
| line_count  | Number of lines in the files (text files)         |
| total_bytes | Total size over all files in bytes                |
| total_size  | Total size over all files as human readable bytes |
