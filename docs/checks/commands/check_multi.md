---
title: multi
---

## check_multi

Runs multiple checks and aggregates their status, output and performance data.

In order to use this plugin, you need to enable 'CheckMulti' in the '[/modules]' section of the snclient.ini.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Inline Checks

    check_multi "check=check_process process=123" "check=check_process process=345" "warn=none" "crit=ok_count ne 2"
    OK - 2 plugins checked, 2 ok

### Config Section Checks

Define checks in `snclient.ini` under `[/settings/check/multi/<name>]`:

    [/settings/check/multi/mycheck]
    check_process process=123
    check_process process=345

Run the configured multi check:

    check_multi "config=mycheck" "warn=none" "crit=ok_count ne 2"
    OK - 2 plugins checked, 2 ok

### External Script Config

    [/settings/check/multi/custom]
    /opt/script/test.sh -H 123
    /opt/script/test2.sh -W 123

Run the configured multi check:

    check_multi "config=custom" "warn=problem_count gt 0"

## Check Specific Arguments

| Argument | Default | Description |
| --- | --- | --- |
| check | | Inline check command to execute (can be specified multiple times) |
| config | | Config section name under `/settings/check/multi/` to execute |

## Attributes

| Filter / Threshold | Default | Description |
| --- | --- | --- |
| warn | `warning_count > 0` | Warning threshold |
| crit | `critical_count > 0 \|\| unknown_count > 0` | Critical threshold |
| count | | Total number of checks executed |
| ok_count | | Number of checks in OK state |
| warning_count | | Number of checks in WARNING state |
| critical_count | | Number of checks in CRITICAL state |
| unknown_count | | Number of checks in UNKNOWN state |
| problem_count | | Number of checks in non-OK state |
| name | | Name/tag of the check |
| command | | Command executed |
| state | | Exit code of the check (0=OK, 1=WARNING, 2=CRITICAL, 3=UNKNOWN) |
| status | | Status text of the check (OK, WARNING, CRITICAL, UNKNOWN) |
| output | | Output of the check |
