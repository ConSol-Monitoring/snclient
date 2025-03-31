---
title: pdh
---

## check_pdh

Checks pdh paths Handles WildCard Expansion

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux | FreeBSD | MacOSX |
|:------------------:|:-----:|:-------:|:------:|
| :white_check_mark: |       |         |        |

## Examples

### Default Check

		check_pdh "counter=foo" "warn=value > 80" "crit=value > 90"
		Everything looks good
		'foo value'=18;80;90

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_pdh
        use                  generic-service
        check_command        check_nrpe!check_pdh!counter=\\System\\System Up Time" "warn=value > 5" "crit=value > 9999
    }

## Argument Defaults

| Argument      | Default Value                                                            |
| ------------- | ------------------------------------------------------------------------ |
| empty-state   | 3 (UNKNOWN)                                                              |
| empty-syntax  | %(status) - No counter found                                             |
| top-syntax    | %(status) - %(problem_count)/%(count) counter (%(count)) %(problem_list) |
| ok-syntax     | %(status) - All %(count) counter values are ok                           |
| detail-syntax | %(name)                                                                  |

## Check Specific Arguments

| Argument     | Description                                                                                            |
| ------------ | ------------------------------------------------------------------------------------------------------ |
| counter      | The fully qualified counter name                                                                       |
| english      | Using English names regardless of system language. Requires Windows Vista or higher                    |
| expand-index | Should indices be translated?                                                                          |
| host         | The name of the host machine in network where the counter should be searched, defaults to local machine |
| instances    | Expand wildcards and fetch all instances                                                               |
| type         | This can be large or float depending what you expect, default is large                                 |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Description                                                        |
| --------- | ------------------------------------------------------------------ |
| count     | Number of items matching the filter. Common option for all checks. |
| value     | The counter value (either float or int)                            |
