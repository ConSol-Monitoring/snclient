# check_service

## Description

Checks the the state of a service on the host

## Thresholds

- status

Available status:
- stopped
- dead
- startpending
- stoppending
- running
- started

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
            check_command           check_nrpe!check_service!'service=Dhcp' 'crit=status = dead'
    }

#### Return

    %> Service is ok.|'Dhcp'=4