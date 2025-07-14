---
title: Common Check Arguments
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
| [ok](#ok)                       | Threshold when to generate an ok state |
| [top-syntax](#top-syntax)       | Top level syntax. |
| [ok-syntax](#ok-syntax)         | Syntax used for ok states. |
| [empty-syntax](#empty-syntax)   | Template syntax used when no item matches a filter. |
| [empty-state](#empty-state)     | Status to return when no items matches the filter. |
| [detail-syntax](#detail-syntax) | Detailed syntax for list items. |
| [perf-syntax](#perf-syntax)     | Performance data syntax. |
| [perf-config](#perf-config)     | Performance data tweaks. |

### Filter

Filter for items which will be included in the check. Unwanted items will be ignored
and won't trigger a warning or critical state.

ex.:

    'filter=service = snclient'

Filter are explained in detail here: [check filter](../filter/)

### Filter+

Extend default filter.

ex.:

    'filter+=service = snclient'

### Warning

Filter which sets a threshold when to generate a warning state. If any wanted item
matches this filter the return state will be escalated to warning.

The syntax works the same way as [filter](#filter) except matching items are not
removed but escalate the status to warning state.

ex.:

    'warn=load > 90%'

Syntax is explained in details on the [expressions page](../expressions).

### Warning+

Extend default warning threshold.

ex.:

    'warn+=load > 90%'

### Critical

Filter which sets a threshold when to generate a critical state. If any wanted item
matches this filter the return state will be escalated to critical.

The syntax works the same way as [filter](#filter) except matching items are not
removed but escalate the status to critical state.

ex.:

    'crit=load > 98%'

Syntax is explained in details on the [expressions page](../expressions).

### Critical+

Extend default critical threshold.

ex.:

    'crit+=load > 90%'

### Ok

Filter which sets a threshold when to generate an ok state. If any wanted item
matches this filter its state will be reset to ok regardless of its previous state.

The syntax works the same way as [filter](#filter) except matching items are not
removed but the status is reset to ok state.

ex.:

    'ok=enabled = 0'

Syntax is explained in details on the [expressions page](../expressions).

### Empty-State

Status to be returned when no item matches the filter. If no filter is given this won't happen.
Empty-State won't be used at all if there is a `count` warning/critical/ok threshold used.

ex.:

    'empty-state=3'

### Top-Syntax

Sets the output format template for the return message. Can include text as well
as special keywords that will be replaced by information from the check.

Details are explained on the [template syntax page](../syntax/).

ex.:

    'top-syntax=%(status) - %(crit_list)'

### Ok-Syntax

Overrides the `top-syntax` if the state is OK. Can include text as well as special
keywords that will be replaced by information from the check.

Details are explained on the [template syntax page](../syntax/).

ex.:

    'ok-syntax=%(status) - everything is fine'

### Empty-Syntax

Sets the format for the return message if no item matched the filter. Overrides the
`top-syntax` template for empty lists.

Details are explained on the [template syntax page](../syntax/).

ex.:

    'empty-syntax=%(status) - nothing found'

### Detail-Syntax

Sets the format for each individual list item in the message.

Details are explained on the [template syntax page](../syntax/).

ex.:

    'detail-syntax=%(name): Memory: %(mem:h) - CPU: %(cpu)%'

### Perf-Syntax

Sets the format for the base names of the performance data. The default is `%(key)`.

ex.:

    'perf-syntax=%(key | lc)'

### Perf-Config

Apply tweaks to performance data, like unit conversion.

`perf-config` syntax is explained in detail here: [Perf Config](../perfconfig/)

ex.:

    'perf-config=used(unit:G)'
