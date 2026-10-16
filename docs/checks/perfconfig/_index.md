---
title: Performance Data Configuration
---

Sometimes you might want to tweak performance data and therefore all checks
support the `perf-config` argument to apply tweaks to them.

## Syntax

```bash
check_plugin perf-config="selector(key:value;...)"
```

For example:

```bash
check_drivesize "perf-config= used %(ignored:true) used(unit:G)"
```

This will convert used disk space bytes into gigabytes. It also removes the percent
performance data from the output completely.

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

The perf-config selector supports wildcards, so something like this will work:

```bash
check_plugin perf-config="*(unit:G)"
```

SNClient will use the options in order and apply the first matching configuration, for example:

```bash
check_plugin perf-config="used(ignored:true) *(unit:G)"
```

will hide `used` performance data and apply unit G to everything else.

## Units

There are several useful conversions available.

### Bytes

The base unit `B` for bytes can be converted into more human-readable units.

A common pattern is `*(unit:G)` to simply convert all performance data into gigabytes.

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

Durations with the base unit `s` for seconds can be converted into the following units:

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

Performance data with a value, a minimum, and a maximum can be converted to a percentage.

```bash
check_plugin perf-config="*(unit:%)"
```
