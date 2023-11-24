---
title: Perfomance Data Configuration
---

Sometimes you might want to tweak performance data and therefore all checks
support the `perf-conf` argument to apply tweaks to them.

## Syntax

    check_plugin perf-config="selector(key:value;...)"

For example:

    check_drivesize "perf-config=used(unit:G) used %(ignored:true)"

This will convert used disk space bytes into gigabytes. It als removes the percent
performance data from the output completely.

## Configuration

The following keys are available:

| Key | Value | Description |
| -------- | ------| ----------- |
`ignored` | `true` or `false` | Remove the performance value from the list if true.
`prefix`  | `string`          | Change the prefix to something else.
`suffix`  | `string`          | Change the suffix to something else.
`unit`    | `string`          | Change the unit to something else.
`magic`   | `number`          | Apply magic factor to performance data.

## Selector

The perf-conf selector supports wildcards, so something like this will work:

    check_plugin perf-config="*(unit:G)"

The snclient will use the options in order and apply the first matching configuration, so for ex.:

    check_plugin perf-config="used(ignored:true) *(unit:G)"

will hide `used` performance data and apply unit G to everything else.

## Units

There are several possible and useful conversion available.

### Bytes

The base unit `B` for bytes can be converted into more human readable units.

A common pattern is `*(unit:G)` to simply convert all performance data into gigabytes.

    check_plugin perf-config="*(unit:G)"

You can choose from these units:

| Unit | Description |
| ---- | ----------- |
K    | KByte
KB   | KByte
KiB  | KiByte
Kb   | KiByte
KI   | KiByte
M    | MByte
MB   | MByte
MiB  | MiByte
Mb   | MiByte
MI   | MiByte
G    | GByte
GB   | GByte
GiB  | GiByte
Gb   | GiByte
GI   | GiByte
T    | TByte
TB   | TByte
TiB  | TiByte
Tb   | TiByte
TI   | TiByte
P    | PByte
PB   | PByte
PiB  | PiByte
Pb   | PiByte
PI   | PiByte
E    | EByte
EB   | EByte
EiB  | EiByte
Eb   | EiByte
EI   | EiByte

### Seconds / Duration

Durations with the base unit `s` for seconds can be converted into the following units:

| Unit | Description |
| ---- | ----------- |
ms     | milliseconds
s      | seconds
m      | minutes
h      | hours
d      | days
w      | weeks
y      | years

For example convert the uptime to days:

    check_uptime "perf-config=*(unit:d)"

### Percent

All performance data which have at least a value and a min and max value can be converted to percent.

    check_plugin perf-config="*(unit:%)"
