---
title: Template Syntax
---

The \*-syntax options can be used to customize the plugin output.

Here is a list of available syntax options which are common for all check plugins.

| Option          | Description |
| --------------- | ----------- |
| `top-syntax`    | Top level syntax |
| `ok-syntax`     | Ok syntax |
| `empty-syntax`  | Template syntax used when no item matches a filter. |
| `detail-syntax` | Detailed syntax for list items. |
| `perf-syntax`   | Performance data syntax |

Templates may contain text, conditionals and macros.

### Macros

Macros can be used to access attributes of the current check. The form is the same
as used in the config file (snclient.ini)

The macro syntax is explained in detail the [macro section](../../configuration/#macros) of
the configuration page.

Supported macro variants:

- `${macroname}`
- `%(macroname)`
- all variants of `$` / `%` and `{}` / `()` are interchangeable

Example:

```bash
check_memory 'detail-syntax=%(type) = %(used)/%(size) (%(used_pct)%)'
```

All macros can also make use of [macro operators](../../configuration/#macro-operators).

For example, the default output applies floating point format to make the output more readable:

```bash
check_memory 'detail-syntax=%(type) = %(used)/%(size) (%(used_pct | fmt=%.1f)%)'
```

The list of available macros for each check is available in the help page of each check.
Another way is to run the check with `-vv` verbose mode, which prints the list details
as well.

### Conditionals

Conditionals can be used to support different output based on expressions.

The basic syntax is:

    {{ IF <expression> }}...{{ ELSIF <expression> }}...{{ ELSE }}...{{ END }}

Example:

```bash
check_service \
    service=sshd \
    detail-syntax='%(name) - {{ IF state == running }}memory: %(rss:h)B cpu: %(cpu:fmt=%.1f)% - age: %(created:age:duration){{ else }}service is $(state){{ end }}' \
    show-all
```

That way, in case of errors a useful message can be displayed and performance details otherwise.

For better readability, here is the template again in multiple lines:

    %(name) -
    {{ IF state == running }}
        memory: %(rss:h)B cpu: %(cpu:fmt=%.1f)% - age: %(created:age:duration)
    {{ else }}
        service is $(state)
    {{ end }}
