---
title: logfile
---

## check_logfile

Checks logfiles or any other text format file for errors or other general patterns

    In order to use this plugin, you need to enable 'CheckLogFile' in the '[/modules]' section of the snclient_local.ini.

    Also, to avoid security issues, you need to set 'allowed pattern' in the '[/settings/check/logfile]'
    section of the snclient_local.ini to a comma separated list of allowed glob patterns.

    Example:
    [/settings/check/logfile]
    allowed pattern  = /var/log/**      # This allows all files recursively in /var/log/
    allowed pattern += /opt/logs/*.log  # This allows all files with .log extension in /opt/logs/

    See https://github.com/bmatcuk/doublestar#patterns for details on the pattern syntax.


- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

Alert if there are errors in the snclient log file:

    check_files files=/var/log/snclient/snclient.log 'warn=line like Warn' 'crit=line like Error'"
    OK - All 1787 / 1787 Lines OK

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
        check_command        check_nrpe!check_logfile!'files=/var/log/snclient/snclient.log' 'warn=line like Warn'
    }

## Argument Defaults

| Argument      | Default Value                                                          |
| ------------- | ---------------------------------------------------------------------- |
| empty-state   | 3 (UNKNOWN)                                                            |
| empty-syntax  | %(status) - No files found                                             |
| top-syntax    | %(status) - %(problem_count)/%(count) lines (%(count)) %(problem_list) |
| ok-syntax     | %(status) - All %(count) / %(total) Lines OK                           |
| detail-syntax | %(line \| chomp \| cut=200)                                            |

## Check Specific Arguments

| Argument     | Description                                                                                           |
| ------------ | ----------------------------------------------------------------------------------------------------- |
| column-split | Tab split default: \t                                                                                 |
| file         | The file that should be checked                                                                       |
| files        | Comma separated list of files                                                                         |
| label        | label:pattern => If the pattern is matched in a line the line will have the label set as detail       |
| line-split   | Character string used to split a file into several lines (default \n)                                 |
| offset       | Starting position (in bytes) for scanning the file (0 for beginning). This overrides any saved offset |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Description                                                        |
| --------- | ------------------------------------------------------------------ |
| count     | Number of items matching the filter. Common option for all checks. |
| filename  | The name of the file                                               |
| line      | Match the content of an entire line                                |
| columnN   | Match the content of the N-th column only if enough columns exists |
