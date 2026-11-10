---
title: Filters
---

Filters select objects from a result list. If you specify a filter, SNClient uses only matching items in the final result.

The warning/critical/ok thresholds use the same syntax. A matching threshold sets the corresponding status.

## Syntax

All filters follow the syntax `<attribute> <operator> <value>`.
See [the operator list](../expressions) for the allowed operators and their meanings.

Each [check plugin](../plugins) documents its available attributes.

[Common attributes](#common-filter-attributes) are listed at the end of this page.

For example:

    filter="status = 'started'"

See the [expressions page](../expressions) for syntax details.

SNClient combines multiple filters with a logical `OR`.

For example:

    filter="drive = 'C:'" filter="drive = 'D:'"

## Filter As Excludes

By default, filters form an inclusion list. With the appropriate operator,
they can also exclude items.

For example:

    filter="fstype not in ('tmpfs', 'debugfs')"

## Default Filter

Some checks have a default filter. SNClient uses it if you do not supply a filter as a check argument.

Each [check plugin](../plugins) documents its default filters.

To unset the default filter, use a `none` filter. For example:

    filter="none"

A new filter replaces the default filter.

## Extending Filters

To extend a default filter, use the `filter+="..."` syntax. For example:

    filter+="status = 'started'"

When you extend the default filter, SNClient combines all filters with a logical `AND`.

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

See the [expressions page](../expressions) for details.

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
