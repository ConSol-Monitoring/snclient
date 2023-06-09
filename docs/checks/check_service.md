# check_service

Check the state of one or more of the computer services.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Metrics](#metrics)

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
        command_line    $USER1$/check_nrpe -2 -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
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
| computer | | Name of the remote computer to check |
| service | | Name of the service to check (set to * to check all services) |
| exclude | | List of services to exclude from the check (mainly used when service is set to *) |
| type | service | The types of services to enumerate? |
| state | all | The states of services to enumerate. (Available states are active, inactive or all)? |


## Metrics

#### **Check specific metrics**

| Metric | Description |
| --- | --- |
| name | The name of the service |
| state | The state of the service |
| desc | Description of the service |
| delayed | If the service is delayed |
| classification | Classification of the service |
| pid | The pid of the service |
| start_type | The configured start type |