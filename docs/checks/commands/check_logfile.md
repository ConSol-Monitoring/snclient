---
title: logfile
---

## check_logfile

Checks logfiles or any other text format file for errors or other general patterns

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_logfile
        use                  generic-service
        check_command        check_nrpe!check_logfile!
    }

## Argument Defaults

| Argument      | Default Value                                                          |
| ------------- | ---------------------------------------------------------------------- |
| empty-state   | 3 (UNKNOWN)                                                            |
| empty-syntax  | %(status) - No files found                                             |
| top-syntax    | %(status) - %(problem_count)/%(count) lines (%(count)) %(problem_list) |
| ok-syntax     | %(status) - All %(count) / %(total) Lines OK                           |
| detail-syntax | %(line)                                                                |

## Check Specific Arguments

| Argument     | Description                                                                                     |
| ------------ | ----------------------------------------------------------------------------------------------- |
| column-split | Tab slit default: \t                                                                            |
| file         | The file that should be checked                                                                 |
| files        | Comma separated list of files                                                                   |
| label        | label:pattern => If the pattern is matched in a line the line will have the label set as detail |
| line-split   | Character string used to split a file into several lines (default \n)                           |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Description                                                        |
| --------- | ------------------------------------------------------------------ |
| count     | Number of items matching the filter. Common option for all checks. |
| filename  | The name of the file                                               |
| line      | Match the content of an entire line                                |
| columnN   | Match the content of the N-th column only if enough columns exists |
