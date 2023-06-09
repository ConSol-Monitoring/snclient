# check_uptime

Check time since the host last booted.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Metrics](#metrics)

## Examples

### **Default check**

    check_uptime
    OK: uptime: 5w 6d 18:19h, boot: 2023-04-28 15:15:42 (UTC)

### Example using **NRPE** and **Naemon**

Naemon Config

    define command{
        command_name    check_nrpe
        command_line    $USER1$/check_nrpe -2 -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
            host_name               testhost
            service_description     check_uptime_testhost
            check_command           check_nrpe!check_uptime!'warn=uptime < 180s' 'crit=uptime < 60s'
    }

Return

    OK: uptime: 5w 6d 18:19h, boot: 2023-04-28 15:15:42 (UTC)

## Argument Defaults

| Argument | Default Value |
| --- | --- |
warning | uptime < 2d |
critical | uptime < 1d |
top-syntax | \${status}: ${list} |
detail-syntax | uptime: \${uptime}, boot: ${boot} (UTC) |

## Metrics

#### **Check specific metrics**

| Metric | Description |
| --- | --- |
| boot | System boot time |
| uptime | Time since last boot |