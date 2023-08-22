---
title: Perfomance Data Configuration
---

# Perfomance Data Configuration

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


## Selector

The perf-conf selector supports wildcards, so something like this will work:

	check_plugin perf-config="*(unit:G)"

The snclient will use the options in order and apply the first matching configuration, so for ex.:

	check_plugin perf-config="used(ignored:true) *(unit:G)"

will hide `used` performance data and apply unit G to everything else.
