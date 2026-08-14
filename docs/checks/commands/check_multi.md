---
title: multi
---

## check_multi

Runs multiple checks and aggregates their status, output and performance data.

	By default 'CheckMulti' is enabled, but you can disable it in the '[/modules]' section of the snclient_local.ini.
	You can also set a limit for the number of checks that can be executed in the '[/settings/check/multi]' section
	of the snclient_local.ini.

	When using the inline mode, you can only use available commands (run 'check_index' to get a full list).

	You can also define custom check sections in the config file, for example:
    [/settings/check/multi/mycheck]
    check_process process=123
    check_process process=345

    This can be executed with 'check_multi "config=mycheck"'.

	It's also possible to use custom scripts in the config section, for example:
	[/settings/check/multi/myscript]
	/path/to/plugin1
	/path/to/plugin2
	/path/to/plugin3

	This can be executed with 'check_multi "config=myscript"'.


- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_multi "check=check_process 'process=firefox'" "check=check_memory 'crit=used_pct gt 80%'"
	OK - 2 plugins checked, 2 ok | 'check_process::count'=1;;;0 'check_process::rss'=258686976B;;;0 ...
	[ 1] check_process OK - all 1 processes are ok.
	[ 2] check_memory OK - physical = 12.22 GiB/16.00 GiB (76.4%), swap = 1.95 GiB/3.00 GiB (65.0%)

	You can define warning/critical conditions based on the number of checks in a certain state (see attributes below):

	check_multi "check=check_dummy 0 'OK'" "check=check_dummy 1 'WARNING'" "critical=problem_count gt 0"
	CRITICAL - 2 plugins checked: 1 ok, 1 warning, 0 critical, 0 unknown
	[ 1] check_dummy OK
	[ 2] check_dummy WARNING

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_multi
        use                  generic-service
        check_command        check_nrpe!check_multi!
    }

## Argument Defaults

| Argument      | Default Value                                                                                         |
| ------------- | ----------------------------------------------------------------------------------------------------- |
| warning       | warning_count > 0                                                                                     |
| critical      | critical_count > 0 \|\| unknown_count > 0                                                             |
| empty-state   | 3 (UNKNOWN)                                                                                           |
| empty-syntax  | %(status) - no checks executed                                                                        |
| top-syntax    | %(status) - %(count) plugins checked: %(ok_count) ok, %(warning_count) warning, %(critical_count) critical, %(unknown_count) unknown%(problem_list) |
| ok-syntax     | %(status) - %(count) plugins checked, %(ok_count) ok                                                  |
| detail-syntax | [%(status)] %(name): %(output)                                                                        |

## Check Specific Arguments

| Argument | Description                                                              |
| -------- | ------------------------------------------------------------------------ |
| check    | Check command to execute (can be specified multiple times)               |
| config   | Config section name under [/settings/check/multi/< section >] to execute |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute      | Description                                                     |
| -------------- | --------------------------------------------------------------- |
| count          | Total number of checks executed                                 |
| ok_count       | Number of checks in OK state                                    |
| warning_count  | Number of checks in WARNING state                               |
| critical_count | Number of checks in CRITICAL state                              |
| unknown_count  | Number of checks in UNKNOWN state                               |
| problem_count  | Number of checks in non-OK state                                |
| name           | Name/tag of the check                                           |
| command        | Command executed                                                |
| state          | Exit code of the check (0=OK, 1=WARNING, 2=CRITICAL, 3=UNKNOWN) |
| status         | Status text of the check (OK, WARNING, CRITICAL, UNKNOWN)       |
| output         | Output of the check                                             |
