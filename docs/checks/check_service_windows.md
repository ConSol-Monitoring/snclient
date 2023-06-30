---
title: check_service (Windows)
---

# check_service (Windows)

Check the state of one or more of the windows computer services.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Metrics](#metrics)

### Implementation

| Windows | Linux | FreeBSD | MacOSX |
|:-------:|:-----:|:-------:|:------:|
|  :white_check_mark:  |  :x:  |  :x:  |  :x:  |

There is a [check_service for linux](check_service_linux) as well.

## Examples

### **Default check**

    check_service
    OK: All 15 service(s) are ok.

Checking a single service:

    check_service service=dhcp
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
            check_command           check_nrpe!check_service!'service=dhcp' 'crit=status = dead'
    }

Return

    OK: All 1 service(s) are ok.


## Argument Defaults

| Argument | Default Value |
| --- | --- |
filter | none |
warning | state != 'running' && start_type = 'delayed' |
critical | state != 'running' && start_type = 'auto' |
empty-state | 3 (Unknown) |
top-syntax | %(status): %(crit_list), delayed (%(warn_list)) |
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
| state | The state of the service, one of: stopped, starting, stopping, running, continuing, pausing, paused or unknown |
| desc | Description of the service |
| delayed | If the service is delayed, can be 0 or 1 |
| classification | Classification of the service, one of: kernel-driver, system-driver, service-adapter, driver, service-own-process, service-shared-process, service or interactive |
| pid | The pid of the service |
| start_type | The configured start type, one of: boot, system, delayed, auto, demand, disabled or unknown |