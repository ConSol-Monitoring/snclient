---
title: load
---

## check_load

Checks the cpu load metrics.

- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows             | Linux               | FreeBSD             | MacOSX              |
|:-------------------:|:-------------------:|:-------------------:|:-------------------:|
| :white_check_mark:  | :white_check_mark:  | :white_check_mark:  | :white_check_mark:  |

## Argument Defaults

| Argument      | Default Value                                       |
| ------------- | --------------------------------------------------- |
| filter        | none                                                |
| empty-state   | 0 (OK)                                              |
| empty-syntax  |                                                     |
| top-syntax    | ${status}: ${list}                                  |
| ok-syntax     |                                                     |
| detail-syntax | ${type} load average: ${load1}, ${load5}, ${load15} |

## Check Specific Arguments

| Argument            | Description                                                           |
| ------------------- | --------------------------------------------------------------------- |
| -c\|--critical      | Critical threshold: CLOAD1,CLOAD5,CLOAD15                             |
| -n\|--procs-to-show | Number of processes to show when printing the top consuming processes |
| -r\|--percpu        | Divide the load averages by the number of CPUs                        |
| -w\|--warning       | Warning threshold: WLOAD1,WLOAD5,WLOAD15                              |

## Attributes

### Check Specific Attributes

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Default | Description                              |
| --------- | ------- | ---------------------------------------- |
| type      |         | type will be either 'total' or 'scaled'  |
| load1     |         | average load value over 1 minute         |
| load5     |         | average load value over 5 minutes        |
| load15    |         | average load value over 15 minutes       |
| load      |         | maximum value of load1, load5 and load15 |
