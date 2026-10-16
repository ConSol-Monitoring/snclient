---
title: Common arguments
---

| Option                          | Description |
| ------------------------------- | ----------- |
| [filter](#filter)               | Filter for which items to check |
| [filter+](#filter-1)            | Same as filter but extend default filter instead of overwriting it. |
| [warning](#warning)             | Threshold when to generate a warning state |
| [warn](#warning)                | Short alias for warning |
| [warning+](#warning-1)          | Same as warning but extend default warning instead of overwriting it |
| [critical](#critical)           | Threshold when to generate a critical state |
| [crit](#critical)               | Short alias for critical |
| [critical+](#critical-1)        | Same as critical but extend default critical instead of overwriting it |
| [ok](#ok)                       | Threshold when to generate an OK state |
| [top-syntax](#top-syntax)       | Top-level syntax. |
| [ok-syntax](#ok-syntax)         | Syntax used for OK states. |
| [empty-syntax](#empty-syntax)   | Template syntax used when no item matches a filter. |
| [empty-state](#empty-state)     | Status to return when no items match the filter. |
| [detail-syntax](#detail-syntax) | Detailed syntax for list items. |
| [perf-syntax](#perf-syntax)     | Performance data syntax. |
| [perf-config](#perf-config)     | Performance data tweaks. |

### Filter

Filter for items which will be included in the check. Unwanted items will be ignored
and won't trigger a warning or critical state.

For example:

    'filter=service = snclient'

Filters are explained in detail here: [check filter](../filter/)

### Filter+

Extend the default filter.

For example:

    'filter+=service = snclient'

### Warning

Expression that defines when to return a WARNING state. If any selected item
matches this expression, the return state will be escalated to WARNING.

The syntax works the same way as [filter](#filter) except matching items are not
removed, but instead set the status to WARNING.

For example:

    'warn=load > 90%'

Syntax is explained in detail on the [expressions page](../expressions).

### Warning+

Extend the default warning threshold.

For example:

    'warn+=load > 90%'

### Critical

Expression that defines when to return a CRITICAL state. If any selected item
matches this expression, the return state will be escalated to CRITICAL.

The syntax works the same way as [filter](#filter) except matching items are not
removed, but instead set the status to CRITICAL.

For example:

    'crit=load > 98%'

Syntax is explained in detail on the [expressions page](../expressions).

### Critical+

Extend the default critical threshold.

For example:

    'crit+=load > 90%'

### OK

Expression that defines when to return an OK state. If any selected item
matches this expression, its state will be reset to OK regardless of its previous state.

The syntax works the same way as [filter](#filter) except matching items are not
removed but the status is reset to OK.

For example:

    'ok=enabled = 0'

Syntax is explained in detail on the [expressions page](../expressions).

### Empty-State

Status to be returned when no item matches the filter. If no filter is given this won't happen.
Empty-State won't be used at all if a `count` warning/critical/ok threshold is used.

For example:

    'empty-state=3'

### Top-Syntax

Sets the output format template for the return message. Can include text as well
as special keywords that will be replaced by information from the check.

Details are explained on the [template syntax page](../syntax/).

For example:

    'top-syntax=%(status) - %(crit_list)'

### OK-Syntax

Overrides the `top-syntax` if the state is OK. Can include text as well as special
keywords that will be replaced by information from the check.

Details are explained on the [template syntax page](../syntax/).

For example:

    'ok-syntax=%(status) - everything is fine'

### Empty-Syntax

Sets the format for the return message if no item matched the filter. Overrides the
`top-syntax` template for empty lists.

Details are explained on the [template syntax page](../syntax/).

For example:

    'empty-syntax=%(status) - nothing found'

### Detail-Syntax

Sets the format for each individual list item in the message.

Details are explained on the [template syntax page](../syntax/).

For example:

    'detail-syntax=%(name): Memory: %(mem:h) - CPU: %(cpu)%'

### Perf-Syntax

Sets the format for the base names of the performance data. The default is `%(key)`.

For example:

    'perf-syntax=%(key | lc)'

### Perf-Config

Apply tweaks to performance data, like unit conversion.

`perf-config` syntax is explained in detail here: [Perf Config](../perfconfig/)

For example:

    'perf-config=used(unit:G)'
