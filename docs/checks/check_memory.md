---
title: check_memory
---

# check_memory

Check free/used memory on the host.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Metrics](#metrics)

### Implementation

| Windows | Linux | FreeBSD | MacOSX |
| --- | --- | --- | --- |
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### **Default check**

    check_memory
    OK: physical = 12 GB, committed = 17 GB |'physical'=11639271424B;;;0;17032155136

Changing the return syntax to get more information:

    check_memory "top-syntax=${list}" "detail-syntax=${type} free: ${free} used: ${used}size: ${size}"
    physical free: 5 GB used: 12 GB size: 17 GB, committed free: 10 GB used: 17 GB size: 27 GB |'physical'=11546886144B;;;0;17032155136


### Example using **NRPE** and **Naemon**

Naemon Config

    define command{
        command_name    check_nrpe
        command_line    $USER1$/check_nrpe -2 -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
            host_name               testhost
            service_description     check_memory_testhost
            check_command           check_nrpe!check_memory!'warn=used > 80%' 'crit=used > 90%'
    }

Return

    OK: physical = 12 GB, committed = 17 GB |'physical'=11639271424B;;;0;17032155136

## Argument Defaults

| Argument | Default Value |
| --- | --- |
warning | used > 80% |
critical | used > 90% |
top-syntax | \${status}: ${list} |
detail-syntax | %(type) = %(used) |

## Metrics

#### **Check specific metrics**

| Metric | Description |
| --- | --- |
| free | Free Memory in bytes (IEC/SI/%) |
| free_pct | Free memory in pct |
| used | Used Memory in bytes (IEC/SI/%) |
| used_pct | Used memory in pct |
| size | Total size of memory |
| type | The type of memory |