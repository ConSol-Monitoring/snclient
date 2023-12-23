---
title: mount
---

## check_mount

Checks the status for a mounted filesystem

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_mount mount=/ options=rw,relatime fstype=ext4
    OK - mounts are as expected

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_mount
        use                  generic-service
        check_command        check_nrpe!check_mount!'mount=/' 'options=rw,relatime'
    }

## Argument Defaults

| Argument      | Default Value                                         |
| ------------- | ----------------------------------------------------- |
| warning       | issues != ''                                          |
| critical      | issues like 'not mounted'                             |
| empty-state   | 3 (UNKNOWN)                                           |
| empty-syntax  | check_mount failed to find anything with this filter. |
| top-syntax    | \${status} - \${problem_list}                         |
| ok-syntax     | \${status} - mounts are as expected                   |
| detail-syntax | mount \${mount} \${issues}                            |

## Check Specific Arguments

| Argument | Description                 |
| -------- | --------------------------- |
| fstype   | The fstype to expect        |
| mount    | The mount point to check    |
| options  | The mount options to expect |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Description            |
| --------- | ---------------------- |
| mount     | Path of mounted folder |
| options   | Mount options          |
| device    | Device of this mount   |
| fstype    | FS type for this mount |
| issues    | Issues found           |
