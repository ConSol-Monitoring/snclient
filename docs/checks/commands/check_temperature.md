---
title: temperature
---

## check_temperature

Check temperature sensors.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_temperature
    OK - Package id 0: 65.0 °C, Core 0: 62.0 °C, Core 1: 61.0 °C, Core 2: 65.0 °C |...

Show all temperature sensors and apply custom thresholds:

    check_temperature filter=none warn="temperature > 85" crit="temperature > 90"
    OK - Package id 0: 65.0 °C, Core 0: 62.0 °C, Core 1: 61.0 °C, Core 2: 65.0 °C |...

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_temperature
        use                  generic-service
        check_command        check_nrpe!check_temperature!
    }

## Argument Defaults

| Argument      | Default Value                                     |
| ------------- | ------------------------------------------------- |
| filter        | temperature != 0 and temperature != 1             |
| warning       | temperature < \${min} \|\| temperature > \${crit} |
| critical      | temperature < \${min} \|\| temperature > \${crit} |
| empty-state   | 3 (UNKNOWN)                                       |
| empty-syntax  | check_temperature failed to find any sensors.     |
| top-syntax    | \${status} - \${list}                             |
| ok-syntax     |                                                   |
| detail-syntax | \${sensor}: \${temperature:fmt=%.1f} °C           |

## Check Specific Arguments

| Argument | Description           |
| -------- | --------------------- |
| sensor   | Show this sensor only |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Description                                    |
| --------- | ---------------------------------------------- |
| sensor    | full name of this sensor, ex.: coretemp_core_0 |
| name      | name of this sensor, ex.: coretemp             |
| label     | label for this sensor, ex.: core 0             |
| value     | current temperature                            |
| crit      | critical value supplied from sensor            |
| max       | max value supplied from sensor                 |
| min       | min value supplied from sensor                 |
