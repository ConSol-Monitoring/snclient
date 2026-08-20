---
title: multi
---

## check_multi

Runs multiple checks and aggregates their status, output and performance data.

	By default 'CheckMulti' is enabled, but you can disable it in the '[/modules]' section of the snclient_local.ini.
	You can also set 'max checks' in the '[/settings/check/multi]' section of the snclient_local.ini, which limits
	the number of checks that can be configured.

	When using the inline mode, you can only use available commands (run 'check_index' to get a full list).

	You can also define custom check sections in the config file, for example:
  [/settings/check/multi/mycheck]
  command[alias1] = check_process process=123
  command[alias2] = check_process process=345

  This can be executed with 'check_multi "config=mycheck"'.

	It's also possible to use custom scripts in the config section, for example:
	[/settings/check/multi/myscript]
	command[alias1] = /path/to/plugin1
	command[alias2] = /path/to/plugin2
	command[alias3] = /path/to/plugin3

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

    check_multi "command[check_process]=check_process 'process=firefox'" "command[check_memory]=check_memory 'type=physical' 'crit=used_pct gt 80%'"
	OK - 2 plugins checked, 2 ok |'check_process::count'=1;;;0 ... 'check_memory::physical %'=78.7%;;;0;100
	[check_process] OK - all 1 processes are ok.
	[check_memory] OK - physical = 12.59 GiB/16.00 GiB (78.7%)

	You can define 'warning' and 'critical' conditions based on the number of checks in a certain state (see attributes below):

	check_multi "command[check_dummy1]=check_dummy 0 'OK - check works'" "command[check_dummy2]=check_dummy 1 'WARNING - problem found'" "critical=problem_count gt 0"
	CRITICAL - 2 plugins checked: 1 ok, 1 warning, 0 critical, 0 unknown - warning(check_dummy2: WARNING - problem found)
	[check_dummy1] OK - check works
	[check_dummy2] WARNING - problem found

	You can also override the 'top-syntax' and use IF ELSE statements to get a certain output based on the results:

	check_multi "command[check_dummy1]=check_dummy 0 'OK'" "command[check_dummy2]=check_dummy 2 'CRITICAL'" \
				"top-syntax={{ if ok_count gt 0 }}OK - %(ok_count)/%(count) checks are OK {{ ELSE }}CRITICAL - all checks failed{{ END }}"
	OK - 1/2 checks are OK
	[check_dummy1] OK
	[check_dummy2] CRITICAL

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
| critical      | critical_count > 0                                                                                    |
| unknown       | unknown_count > 0                                                                                     |
| empty-state   | 3 (UNKNOWN)                                                                                           |
| empty-syntax  | %(status) - no checks executed                                                                        |
| top-syntax    | %(status) - %(count) plugins checked: %(ok_count) ok, %(warning_count) warning, %(critical_count) critical, %(unknown_count) unknown - %(problem_list) |
| ok-syntax     | {{ if problem_count gt 0 }}%(status) - %(count) plugins checked: %(ok_count) ok, %(warning_count) warning, %(critical_count) critical, %(unknown_count) unknown - %(problem_list){{ ELSE }}%(status) - %(count) plugins checked, %(ok_count) ok{{ END }} |
| detail-syntax | %(name): %(output)                                                                                    |

## Check Specific Arguments

| Argument | Description                                                               |
| -------- | ------------------------------------------------------------------------- |
| command  | Check command to execute with mandatory unique tag, e.g. command[tag]=... |
| config   | Config section name under [/settings/check/multi/< section >] to execute  |

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
| name           | Name/Tag of the check                                           |
| tag            | Alias for name                                                  |
| command        | Command executed                                                |
| state          | Exit code of the check (0=OK, 1=WARNING, 2=CRITICAL, 3=UNKNOWN) |
| status         | Status text of the check (OK, WARNING, CRITICAL, UNKNOWN)       |
| output         | Check output                                                    |
| shortoutput    | First line of the check output                                  |
