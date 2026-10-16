---
title: Macros
---

Macros can be used to modify raw SNClient values.

## Macro Operators

Values can be altered by adding a colon-separated suffix.

Supported operators are:

| Suffix                | Example                               | Description |
| --------------------- | ------------------------------------- | ------------|
| `:lc`                 | Word -> word                          | make value lowercase |
| `:uc`                 | test -> TEST                          | make value uppercase |
| `:h`                  | 1000 -> 1k                            | make a number more readable |
| `:duration`           | 125 -> 2m 5s                          | convert a number of seconds into a human-readable duration |
| `:age`                | 1700834034 -> 374                     | subtract value from current Unix timestamp to get the age in seconds |
| `:date`               | 1700834034 -> 2023-11-24 14:53:54 CET | convert a Unix timestamp into a human-readable date (local timezone) |
| `:utc`                | 1700834034 -> 2023-11-24 13:53:54 UTC | convert a Unix timestamp into a human-readable date (UTC timezone) |
| `:fmt=<fmt>`          | 123.45 -> 123.5                       | apply a format, e.g. $(total \| fmt=%.1f) (using Go's fmt.Sprintf) |
| `:s/regexp/replace/`  | C:\ % -> C                            | apply regexp replacement, e.g. $(name \| 's/\\W//' ) (using Go's regexp.Compile) |
| `:ascii`              | C:\ % -> C                            | remove any non-ASCII characters |
| `:trim`               | " test " -> "test"                    | remove leading and trailing whitespace |
| `:chomp`              | " test " -> " test"                   | remove trailing whitespace only |

Operators can be put together (spaces are optional):

```txt
$(datemacro:date : uc)
```

This converts $(datemacro) to a human-readable date and makes everything uppercase.

You can also use the pipe symbol to use multiple operators in a row, for example:

```txt
$(macroname | date | uc)
```

Use the generic fmt operator to apply a format to numbers, for example:

```txt
$(used_pct | fmt=%d)
```
