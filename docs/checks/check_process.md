---
title: check_process
---

# check_process

Check state/metrics of one or more of the processes running on the computer.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Metrics](#metrics)

### Implementation

| Windows | Linux | FreeBSD | MacOSX |
|:-------:|:-----:|:-------:|:------:|
| :construction: | :construction: | :construction: | :construction: |

## Examples

### **Default check**

    check_process process=explorer.exe
    OK: explorer.exe=started


### Example using **NRPE** and **Naemon**

Naemon Config

    define command{
        command_name    check_nrpe
        command_line    $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
            host_name               testhost
            service_description     check_process_testhost
            check_command           check_nrpe!check_process!'process=explorer.exe' 'warn=none' 'crit=state != started'
    }

Return

    OK: explorer.exe=started

## Argument Defaults

| Argument | Default Value |
| --- | --- |
empty-state | 3 (Unknown) |
top-syntax | %(status): %(problem_list) |
ok-syntax | %(status): ${list} |
empty-syntax | No processes found |
detail-syntax | %(process)=%(state) |

### **Check specific arguments**

| Argument | Description |
| --- | --- |
| process | The process to check, set to * to check all |

## Metrics

#### **Check specific metrics**

| Metric | Description |
| --- | --- |
| process | Name of the process |
| exe | Alias for process |
| state | State of the process |
| command_line | Command line of the process |
| creation | Creation time |
| filename | Name of the process (full path) |
| handles | Number of handles |
| kernel | Kernel time in seconds |
| pagefile | Pagefile usage in bytes |
| peak_pagefile | Peak pagefile usage in bytes |
| peak_virtual | Peak virtual size in bytes |
| peak_working_set | Peak working set size in bytes |
| pid | Process ID |
| user | User time in seconds |
| virtual | Virtual size in bytes |
| working_set | Working set size in bytes |