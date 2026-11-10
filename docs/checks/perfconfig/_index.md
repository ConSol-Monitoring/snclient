---
title: Performance Data Configuration
---

All checks support the `perf-config` argument to adjust performance data.

## Syntax

```bash
check_plugin perf-config="selector(key:value;...)"
```

For example:

```bash
check_drivesize "perf-config= used %(ignored:true) used(unit:G)"
```

This command converts used disk space from bytes to gigabytes.
It also removes the percent performance data from the output.

## Configuration

The following keys are available:

| Key       | Value             | Description |
| --------- | ------------------| ----------- |
| `ignored` | `true` or `false` | Remove the performance value from the list if true. |
| `prefix`  | `string`          | Change the prefix to something else. |
| `suffix`  | `string`          | Change the suffix to something else. |
| `unit`    | `string`          | Change the unit to something else. |
| `magic`   | `number`          | Apply magic factor to performance data. |

## Selector

The perf-config selector supports wildcards:

```bash
check_plugin perf-config="*(unit:G)"
```

SNClient processes options in order and applies the first matching configuration. For example:

```bash
check_plugin perf-config="used(ignored:true) *(unit:G)"
```

This hides `used` performance data and applies unit G to everything else.

## Adding a metric

Some checks calculate values they do not emit as performance data by default.
A perf-config selector can add a known check value as a new performance metric.
Partial wildcards work as in regular selectors.
For example, `perf-config="*wait(unit:%)"` adds every value whose name contains `wait`.

```bash
check_load perf-config="load(unit:)"
```

`check_load` reports `load1`, `load5` and `load15`. It also calculates `load`, the maximum of the three, internally.
The `load(...)` selector adds `load` to the performance data.
It applies the same unit/prefix/suffix/magic adjustments as for any other matched metric.

A bare `*` selector does not add metrics.

## Units

You can convert performance data to the units listed below.

### Bytes

Convert the base unit `B` for bytes to more readable units.

Use `*(unit:G)` to convert all performance data to gigabytes.

```bash
check_plugin perf-config="*(unit:G)"
```

You can choose from these units:

| Unit | Description |
| ---- | ----------- |
| K    | kilobyte       |
| KB   | kilobyte       |
| KiB  | kibibyte      |
| Kb   | kibibyte      |
| KI   | kibibyte      |
| M    | megabyte       |
| MB   | megabyte       |
| MiB  | mebibyte      |
| Mb   | mebibyte      |
| MI   | mebibyte      |
| G    | gigabyte       |
| GB   | gigabyte       |
| GiB  | gibibyte      |
| Gb   | gibibyte      |
| GI   | gibibyte      |
| T    | terabyte       |
| TB   | terabyte       |
| TiB  | tebibyte      |
| Tb   | tebibyte      |
| TI   | tebibyte      |
| P    | petabyte       |
| PB   | petabyte       |
| PiB  | pebibyte      |
| Pb   | pebibyte      |
| PI   | pebibyte      |
| E    | exabyte       |
| EB   | exabyte       |
| EiB  | exbibyte      |
| Eb   | exbibyte      |
| EI   | exbibyte      |

### Seconds / Duration

Convert durations with the base unit `s` for seconds to these units:

| Unit | Description  |
| ---- | ------------ |
| ms   | milliseconds |
| s    | seconds      |
| m    | minutes      |
| h    | hours        |
| d    | days         |
| w    | weeks        |
| y    | years        |

For example, convert the uptime to days:

```bash
check_uptime "perf-config=*(unit:d)"
```

### Percent

If performance data has a value, a minimum, and a maximum, you can convert it to a percentage.

```bash
check_plugin perf-config="*(unit:%)"
```
