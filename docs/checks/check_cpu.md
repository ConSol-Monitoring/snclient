# check_cpu

Checks if the load of the CPU(s) are within bounds.

- [Examples](#examples)
- [Arguments](#arguments)
- [Metrics](#metrics)

## Examples

### **Default check**

    check_cpu
    OK: CPU load is ok. |'total 5m'=13%;80;90 'total 1m'=13%;80;90 'total 5s'=13%;80;90

Checking **each core** by adding filter=none (disabling the filter):

    check_cpu filter=none
    OK: CPU load is ok. |'core4 5m'=13%;80;90 'core4 1m'=12%;80;90 'core4 5s'=9%;80;90 'core6 5m'=10%;80;90 'core6 1m'=10%;80;90 'core6 5s'=3%;80;90 'core5 5m'=10%;80;90 'core5 1m'=9%;80;90 'core5 5s'=6%;80;90 'core7 5m'=10%;80;90 'core7 1m'=10%;80;90 'core7 5s'=7%;80;90 'core1 5m'=13%;80;90 'core1 1m'=12%;80;90 'core1 5s'=10%;80;90 'core2 5m'=17%;80;90 'core2 1m'=17%;80;90 'core2 5s'=9%;80;90 'total 5m'=12%;80;90 'total 1m'=12%;80;90 'total 5s'=8%;80;90 'core3 5m'=12%;80;90 'core3 1m'=12%;80;90 'core3 5s'=11%;80;90 'core0 5m'=14%;80;90 'core0 1m'=14%;80;90 'core0 5s'=14%;80;90


### Example using **NRPE** and **Naemon**

Naemon Config

    define command{
        command_name    check_nrpe
        command_line    $USER1$/check_nrpe -2 -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
            host_name               testhost
            service_description     check_uptime_testhost
            check_command           check_nrpe!check_cpu!'warn=used_pct > 80' 'crit=used_pct > 95'
    }

Return

    OK: CPU load is ok. |'total 5m'=13%;80;90 'total 1m'=13%;80;90 'total 5s'=13%;80;90

## Arguments

| Option | Default Value | Description |
| --- | --- | --- |
[filter](#filter) | core = 'total' | Filter for which items to check.
[warning](#warning) | load > 80 | Threshold when to generate a warning state.
[warn](#warning) | | Short alias for warning.
[critical](#critical) | load > 90 | Threshold when to generate a critical state.
[crit](#critical) | | Short alias for critical.
[ok](#ok) | | Threshold when to generate an ok state.?
[empty-state](#empty-state) | 3 (Unknown) | Status to return when no items matches the filter.
[top-syntax](#top-syntax) | \${status}: ${problem_list} | Top level syntax.
[ok-syntax](#ok-syntax) | %(status): CPU load is ok. | Ok syntax.
[empty-syntax](#empty-syntax) | check_cpu failed to find anything with this filter. | Empty syntax.
[detail-syntax](#detail-syntax) | \${time}: ${load}% | Detailed/Individual Syntax.
[perf-syntax](#perf-syntax) | | Perfdata syntax.?
time | | time to check?

### FILTER:

Filter for items which will be included in the check. Unwanted items will be ignored and wont trigger a warning or critical state.

*Default Value*: core = 'total'

### WARNING:

Filter which sets a threshold when to generate a warning state. If any wanted item matches this filter the return state will be escalated to warning.

*Default Value*: load > 80

### CRITICAL:

Filter which sets a threshold when to generate a critical state. If any wanted item matches this filter the return state will be escalated to critical.

*Default Value*: load > 90

### OK:

Filter which sets a threshold when to generate an ok state. If any wanted item matches this filter its state will be reset to ok regardless of its previous state.

### EMPTY-STATE:

Status to be returned when no item matches the filter. If no filter is given this wont happen.

*Default Value*: ignored

### TOP-SYNTAX:

Sets the format for the return message. Can include text as well as special keywords that will be replaced by information from the check. Keyword Syntax: ´\${keyword} or %(keyword). $ and % as well as {} and () can be used interchangeably.

*Default Value*: \${status}: ${problem_list}

### OK-SYNTAX:

Sets the format for the return message if the state is OK. Can include text as well as special keywords that will be replaced by information from the check. Keyword Syntax: \${keyword} or %(keyword). $ and % as well as {} and () can be used interchangeably.

*Default Value*: ${status}: CPU load is ok.

### EMPTY-SYNTAX:

Sets the format for the return message if no item matched the filter.

### DETAIL-SYNTAX:

Sets the format for each individual item in the message.

*Default Value*: \${time}: ${load}%

### PERF-SYNTAX:

Sets the format for the base names of the performance data.

*Default Value*: \${core} ${time}

## Metrics

#### **Check specific metrics**

| Metric | Description |
| --- | --- |
| core | Core to check (total or core ##) |
| core_id | Core to check (total or core_##)? |
| idle | Current idle load for a given core? |
| kernel | Current kernel load for a given core? |
| load | Current load for a given core
| time | Time frame to check |

#### **Common metrics**

| Metric | Description |
| --- | --- |
| status | The returned status (OK/WARN/CRIT/UNKNOWN) |
| count | Number of items matching the filter. |
| list | List of all items matching the filter. |
| ok_count | Number of items that are ok |
| ok_list | List of items that are ok |
| warn_count | Number of items that matched the warning threshold |
| warn_list | List of items that matched the warning threshold |
| crit_count | Number of items that matched the critical threshold |
| crit_list | List of items that matched the critical threshold |
| problem_count | Number of items that matched either warning or critical threshold |
| problem_list | List of items that matched either warning or critical threshold |