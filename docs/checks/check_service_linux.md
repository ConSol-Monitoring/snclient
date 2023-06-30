---
title: check_service (Linux)
---

# check_service (Linux)

Check the state of one or more of the linux (systemctl) services.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Metrics](#metrics)

### Implementation

| Windows | Linux | FreeBSD | MacOSX |
|:-------:|:-----:|:-------:|:------:|
|  :x:  |  :white_check_mark:  |  :x:  |  :x:  |

There is a [check_service for windows](check_service_windows) as well.

## Examples

### **Default check**

    check_service
    OK: All 15 service(s) are ok.

Checking a single service:

    check_service service=postfix
    OK: All 1 service(s) are ok.


### Example using **NRPE** and **Naemon**

Naemon Config

    define command{
        command_name    check_nrpe
        command_line    $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
            host_name               testhost
            service_description     check_service_testhost
            check_command           check_nrpe!check_service!'service=postfix' 'crit=status != running'
    }

Return

    OK: All 1 service(s) are ok.

## Argument Defaults

| Argument | Default Value |
| --- | --- |
filter | none |
warning | none |
critical | state not in ('running', 'oneshot', 'static') && preset != 'disabled' |
empty-state | 3 (Unknown) |
top-syntax | %(status): %(crit_list) |
ok-syntax | %(status): All %(count) service(s) are ok. |
empty-syntax | %(status): No services found |
detail-syntax | \${name}=\${state} (${start_type}) |

### **Check specific arguments**

| Argument | Default Value | Description |
| --- | --- | --- |
| service | | Name of the service to check (set to * to check all services) |
| exclude | | List of services to exclude from the check (mainly used when service is set to *) |


## Filter

#### **Check specific filter**

| Filter Attribute | Description |
| ---------------- | ----------- |
| name | The name of the service |
| service | Same as name |
| desc | Description of the service |
| state | The state of the service, one of: stopped, starting, oneshot, running or unknown |
| preset | The preset attribute of the service, one of: enabled or disabled |
| pid | The pid of the service |
| mem | The memory usage |