---
title: Filters
---

Filters can be used to select specific objects from a result list. So if a filter
is specified, only those elements matching the filter will be considered when
building the final result.

The same syntax is used for warning/critical/ok thresholds, except when used
as a threshold, the corresponding status will be set.

## Syntax

All filters follow the syntax `<attribute> <operator> <value>`. For a list and explanation
of allowed operators see [the operator list](../expressions).

The list of possible attributes is documented along with each [check plugin](../plugins).

[Common attributes](#common-filter-attributes) are listed at the end of this page.

For example:

    filter="status = 'started'"

Syntax is explained in detail on the [expressions page](../expressions).

Multiple filters can be used and will be combined with a logical `OR`.

For example:

    filter="drive = 'C:'" filter="drive = 'D:'"

## Filter As Excludes

By default, filters form an inclusion list. With the appropriate operator,
they can also exclude items.

For example:

    filter="fstype not in ('tmpfs', 'debugfs')"

## Default Filter

Some checks have a default filter, which will be used if no filter is supplied
as a check argument.

Default filters are documented along with each [check plugin](../plugins).

The default filter can be unset by using a `none` filter, for example:

    filter="none"

The default filter will be overwritten if a new filter is set.

## Extending Filters

An existing default filter can be extended using the `filter+="..."` syntax, for example:

    filter+="status = 'started'"

When extending the default filter, all filters will be combined with a logical `AND`.

For example:

    default filter="fstype is not tmpfs"

    filter += "fstype is not debugfs"

results in:

    fstype is not tmpfs AND fstype is not debugfs

The same applies to appending to previously set filters:

For example:

    filter="fstype is not tmpfs"    filter+="fstype is not debugfs"

also results in:

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
| ok_count      | Number of items that are OK |
| ok_list       | List of items that are OK |
| warn_count    | Number of items that matched the warning threshold |
| warn_list     | List of items that matched the warning threshold |
| crit_count    | Number of items that matched the critical threshold |
| crit_list     | List of items that matched the critical threshold |
| problem_count | Number of items that matched either warning or critical threshold |
| problem_list  | List of items that matched either warning or critical threshold |
