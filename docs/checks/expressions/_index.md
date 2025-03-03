---
title: Expressions
---

Expressions are used for the `filter`, `warning`, `critical` and `ok` arguments.

## Syntax

The basic syntax is `<attribute> <operator> <value>`. For a list and explanation
of allowed operators see [the operator list](#operator).

The list of possible attributes is documented along with each [check plugin](../plugins).

ex.:

    filter="status = 'started'"

    critical="count >= 1"

## Logical Operator

To combine multiple expressions you can use logical operator and brackets.

| Operator | Alias  | Description              | Example                  |
| -------- | -------| ------------------------ | ------------------------ |
| `and`    | `&&`   | Logical **and** operator | count = 0 and state != 1 |
| `or`     | `\|\|` | Logical **or** operator  | used > 5 or free < 20    |

ex.:

    filter="(status = 'started' or status = 'pending') and usage > 5%"

## Operator

| Operator    | Alias / Safe expression     | Types            | Case Sens. | Description |
| ----------- | --------------------------- | -----------------| ---------- | ----------- |
| `=`         | `==`, `is`, `eq`            | Strings, Numbers | Yes        | Matches on **exact equality**, ex.: `status = 'started'` |
| `!=`        | `is not`, `ne`              | Strings, Numbers | Yes        | Matches if value is **not exactly equal**. ex.: `5 != 3` |
| `like`      | `ilike`                     | Strings          | No         | Matches if value contains the condition (**case insensitive substring match**), ex.: `status like "pend"` |
| `unlike`    | `not like`, `not ilike`     | Strings          | No         | Matches if value does **not contain** the **case insensitive substring**, ex.: `status unlike "stopped"` |
| `slike`     | `strictlike`                | Strings          | Yes        | Matches a **case sensitive substring**, ex.: `name slike "WMI"` |
| `not slike` | `not strictlike`            | Strings          | Yes        | Matches if a **case sensitive substring** cannot be found, ex.: `name not slike "WMI"` |
| `~`         | `regex`, `regexp`           | Strings          | Yes        | Performs a **regular expression** match, ex.: `status ~ '^pend'` |
| `!~`        | `not regex`, `not regexp`   | Strings          | Yes        | Performs a **inverse regular expression** match, ex.: `status !~ 'stop'` |
| `~~`        | `regexi`, `regexpi`         | Strings          | No         | Performs a **case insensitive regular expression** match, ex.: `status ~~ '^pend'`. An alternative way is to use `//i` as in `status ~ /^pend/i` |
| `!~~`       | `not regexi`, `not regexpi` | Strings          | No         | Performs a **inverse case insensitive regular expression** match, ex.: `status !~~ 'stop'` |
| `<`         | `lt`                        | Numbers          | -          | Matches **lower than** numbers, ex.: `usage < 5%` |
| `<=`        | `le`, `lte`                 | Numbers          | -          | Matches **lower or equal** numbers, ex.: `usage <= 5%` |
| `>`         | `gt`                        | Numbers          | -          | Matches **greater than** numbers, ex.: `usage > 5%` |
| `>=`        | `ge`, `gte`                 | Numbers          | -          | Matches **greater or equal** numbers, ex.: `usage >= 5%` |
| `in`        |                             | Strings          | Yes        | Matches if element **is in list**  ex.: `status in ('start', 'pending')` |
| `not in`    |                             | Strings          | Yes        | Matches if element **is not in list**  ex.: `status not in ('stopped', 'starting')` |

## Other Operators

For backwards compatibility there more operators available:

| Operator | Alias   | Description     | Example                     |
| -------- | --------| ----------------| --------------------------- |
| `''`     | `str()` | String constant | exe like str(winlogon.exe)  |
