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

Select the items to include in the check. SNClient ignores other items, so they cannot trigger a warning or critical state.

For example:

    'filter=service = snclient'

See [check filter](../filter/) for details.

### Filter+

Extend the default filter.

For example:

    'filter+=service = snclient'

### Warning

This expression defines when to return WARNING.
If any selected item matches, SNClient escalates the return state to WARNING.

The syntax is the same as [filter](#filter). SNClient keeps matching items and sets the status to WARNING.

For example:

    'warn=load > 90%'

See the [expressions page](../expressions) for syntax details.

### Warning+

Extend the default warning threshold.

For example:

    'warn+=load > 90%'

### Critical

This expression defines when to return CRITICAL.
If any selected item matches, SNClient escalates the return state to CRITICAL.

The syntax is the same as [filter](#filter). SNClient keeps matching items and sets the status to CRITICAL.

For example:

    'crit=load > 98%'

See the [expressions page](../expressions) for syntax details.

### Critical+

Extend the default critical threshold.

For example:

    'crit+=load > 90%'

### OK

This expression defines when to return OK.
If any selected item matches, SNClient resets its state to OK regardless of its previous state.

The syntax is the same as [filter](#filter). SNClient keeps matching items and resets the status to OK.

For example:

    'ok=enabled = 0'

See the [expressions page](../expressions) for syntax details.

### Empty-State

This option sets the status to return when no item matches the filter.
Without a filter, this does not happen.
If you use a `count` warning/critical/ok threshold, SNClient ignores Empty-State.

For example:

    'empty-state=3'

### Top-Syntax

This option sets the output template for the return message.
The template can include text and special keywords. SNClient replaces the keywords with information from the check.

See the [template syntax page](../syntax/) for details.

For example:

    'top-syntax=%(status) - %(crit_list)'

### OK-Syntax

This option overrides `top-syntax` when the state is OK.
The template can include text and special keywords. SNClient replaces the keywords with information from the check.

See the [template syntax page](../syntax/) for details.

For example:

    'ok-syntax=%(status) - everything is fine'

### Empty-Syntax

This option sets the return message format when no item matches the filter.
It overrides the `top-syntax` template for empty lists.

See the [template syntax page](../syntax/) for details.

For example:

    'empty-syntax=%(status) - nothing found'

### Detail-Syntax

Sets the format for each individual list item in the message.

See the [template syntax page](../syntax/) for details.

For example:

    'detail-syntax=%(name): Memory: %(mem:h) - CPU: %(cpu)%'

### Perf-Syntax

Sets the format for the base names of the performance data. The default is `%(key)`.

For example:

    'perf-syntax=%(key | lc)'

### Perf-Config

Adjust performance data, for example with unit conversion.

See [Perf Config](../perfconfig/) for the `perf-config` syntax.

For example:

    'perf-config=used(unit:G)'
