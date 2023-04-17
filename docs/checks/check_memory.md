# check_memory

## Description

Checks the hosts physical and committed memory

## Tresholds

- free (B, KB, MB, GB, TB, %)
- used (B, KB, MB, GB, TB, %)
- free_pct
- used_pct

## Examples

### Using NRPE

#### Naemon Config

    define command{
        command_name    check_nrpe
        command_line    $USER1$/check_nrpe -2 -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
            host_name               testhost
            service_description     check_uptime_testhost
            check_command           check_nrpe!check_memory!'warn=used_pct > 80' 'crit=used_pct > 95'
    }

#### Return

    %> committed = 12 GB, physical = 9.0 GB|'committed_used_pct'=60;;;;'physical_used_pct'=52;;;;