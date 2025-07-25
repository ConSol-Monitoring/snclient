﻿---
title: Filter
---

Filter can be uses to select specific objects from a result list. So if a filter
is specified, only those elements matching the filter will be considered when
building the final result.

The same syntax is used for warning/critical/ok thresholds, except when used
as threshold, the corresponding status will be set.

## Syntax

All filter follow the syntax `<attribute> <operator> <value>`. For a list and explanation
of allowed operators see [the operator list](../expressions).

The list of possible attributes is documented along with each [check plugin](../plugins).

[Common attributes](#common-filter-attributes) are listed at the end of this page.

ex.:

    filter="status = 'started'"

Syntax is explained in details on the [expressions page](../expressions).

Multiple filter can be used and will be combined with an logical `OR`.

ex.:

    filter="drive = 'C:'" filter="drive = 'D:'"

## Filter As Excludes

By default filters are a positive include list. But with the right operator
they can be used as exclude as well.

ex.:

    filter="fstype not in ('tmpfs', 'debugfs')"

## Default Filter

Some checks do have default filter, which will be used if no filter is supplied
as check argument.

Default filter are documented along with each [check plugins](../plugins).

The default filter can be unset by using a `none` filter, ex.:

    filter="none"

Default filter will be overwritten if a new filter is set.

## Extending Filters

Existing default filter can be extended by using a `filter+="..."` syntax, ex.:

    filter+="status = 'started'"

In case of extending the default filter, all filter will be combined with an logical `AND`.

ex.:
    default filter="fstype is not tmpfs"

    filter += "fstype is not debugfs"

results in:

    fstype is not tmpfs AND fstype is not debugfs

## Expressions

Expressions are explained on the [expressions page](../expressions).

## Common Filter Attributes

| Attribute     | Description |
| ------------- | ----------- |
| status        | The returned status (OK/WARN/CRIT/UNKNOWN) |
| count         | Number of items matching the filter. |
| total         | Total number of items |
| list          | List of all items matching the filter. |
| ok_count      | Number of items that are ok |
| ok_list       | List of items that are ok |
| warn_count    | Number of items that matched the warning threshold |
| warn_list     | List of items that matched the warning threshold |
| crit_count    | Number of items that matched the critical threshold |
| crit_list     | List of items that matched the critical threshold |
| problem_count | Number of items that matched either warning or critical threshold |
| problem_list  | List of items that matched either warning or critical threshold |
