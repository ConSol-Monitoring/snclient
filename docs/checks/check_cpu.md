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
            service_description     check_cpu_testhost
            check_command           check_nrpe!check_cpu!'warn=load > 80' 'crit=load > 95'
    }

Return

    OK: CPU load is ok. |'total 5m'=13%;80;90 'total 1m'=13%;80;90 'total 5s'=13%;80;90

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