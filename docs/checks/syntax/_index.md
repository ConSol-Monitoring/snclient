---
title: Template Syntax
---

Use the \*-syntax options to customize plugin output.

All check plugins support these syntax options:

| Option          | Description |
| --------------- | ----------- |
| `top-syntax`    | Top-level syntax |
| `ok-syntax`     | Ok syntax |
| `empty-syntax`  | Template syntax used when no item matches a filter. |
| `detail-syntax` | Detailed syntax for list items. |
| `perf-syntax`   | Performance data syntax |

Templates may contain text, conditionals and macros.

### Macros

Use macros to access attributes of the current check. They use the same form as in the configuration file (snclient.ini).

See the [macro section](../../configuration/#macros) of the configuration page for syntax details.

Supported macro variants:

- `${macroname}`
- `%(macroname)`
- all variants of `$` / `%` and `{}` / `()` are interchangeable

Example:

```bash
check_memory 'detail-syntax=%(type) = %(used)/%(size) (%(used_pct)%)'
```

All macros support [macro operators](../../configuration/#macro-operators).

For example, the default output applies floating-point formatting to make the output more readable:

```bash
check_memory 'detail-syntax=%(type) = %(used)/%(size) (%(used_pct | fmt=%.1f)%)'
```

Each check's help page lists its available macros.
You can also run the check in verbose mode with `-vv` to print the list details.

### Conditionals

Use conditionals to change the output based on expressions.

The basic syntax is:

    {{ IF <expression> }}...{{ ELSIF <expression> }}...{{ ELSE }}...{{ END }}

Example:

```bash
check_service \
    service=sshd \
    detail-syntax='%(name) - {{ IF state == running }}memory: %(rss:h)B cpu: %(cpu:fmt=%.1f)% - age: %(created:age:duration){{ else }}service is $(state){{ end }}' \
    show-all
```

This displays a useful message when an error occurs, and performance details otherwise.

The same template on multiple lines:

    %(name) -
    {{ IF state == running }}
        memory: %(rss:h)B cpu: %(cpu:fmt=%.1f)% - age: %(created:age:duration)
    {{ else }}
        service is $(state)
    {{ end }}
