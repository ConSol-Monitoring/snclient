# check_uptime

## Description

Checks the uptime of the host.

## Thresholds

- uptime

Available Units: s

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
            check_command           check_nrpe!check_uptime!'warn=uptime < 180s' 'crit=uptime < 60s'
    }

#### Return

    %> uptime: 5d 3:45h, boot: 2023-03-29 11:38:05 (UTC)|'uptime'=445507s;;;180;60