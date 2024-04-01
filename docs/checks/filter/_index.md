---
title: Filter
---

Filter can be uses to select specific objects from a result list. They are also used
in warning, critical or ok thresholds.

## Syntax

All filter follow the syntax `<attribute> <operator> <value>`. For a list and explanation
of allowed operators see [the operator list](#operator).

The list of possible attributes is documented along with the [check plugins](../plugins).

ex.:

    status = 'started'

## Logical Operator

To combine multiple filter you can use logical operator and brackets.

| Operator | Alias | Description |
| -------- | ------| ----------- |
`and`   | `&&`  | Logical **and** operator
`or`    | `\|\|`  | Logical **or** operator

ex.:

    (status = 'started' or status = 'pending') and usage > 5%

## Operator

| Operator | Alias | Types | Description |
| -------- | ---------------------- | --------| ----------- |
`=`  | `==`, `is`, `eq`             | Strings, Numbers | Matches on **exact equality**, ex.: `status = 'started'`
`!=` | `is not`, `ne`               | Strings, Numbers | Matches if value is **not exactly equal**. ex.: `5 != 3`
`like` |                            | Strings | Matches if value contains the condition (**substring match**), ex.: `status like "pend"`
`unlike` | `not like`               | Strings | Matches if value does **not contain** the **substring**, ex.: `status unlike "stopped"`
`ilike` |                           | Strings | Matches a **case insensitive substring**, ex.: `name ilike "WMI"`
`not ilike` |                       | Strings | Matches if a **case insensitive substring** cannot be found, ex.: `name not ilike "WMI"`
`~` | `regex`, `regexp`             | Strings | Performs a **regular expression** match, ex.: `status ~ '^pend'`
`!~` | `not regex`, `not regexp`    | Strings | Performs a **inverse regular expression** match, ex.: `status !~ 'stop'`
`~~` | `regexi`, `regexpi`          | Strings | Performs a **case insensitive regular expression** match, ex.: `status ~~ '^pend'`. An alternative way is to use `//i` as in `status ~ /^pend/i`
`!~~` | `not regexi`, `not regexpi` | Strings | Performs a **inverse case insensitive regular expression** match, ex.: `status !~~ 'stop'`
`<` | `lt`                          | Numbers | Matches **lower than** numbers, ex.: `usage < 5%`
`<=` | `le`                         | Numbers | Matches **lower or equal** numbers, ex.: `usage <= 5%`
`>` | `gt`                          | Numbers | Matches **greater than** numbers, ex.: `usage > 5%`
`>=` | `ge`                         | Numbers | Matches **greater or equal** numbers, ex.: `usage >= 5%`
`in` |                              | Strings | Matches if element **is in list**  ex.: `status in ('start', 'pending')`
`not in` |                          | Strings | Matches if element **is not in list**  ex.: `status not in ('stopped', 'starting')`
