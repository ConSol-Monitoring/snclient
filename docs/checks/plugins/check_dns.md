---
title: check_dns
---

## check_dns

Runs check_dns to perform nameserver checks.
It basically wraps the plugin from https://github.com/mackerelio/go-check-plugins/tree/master/check-dns

- [Examples](#examples)
- [Usage](#usage)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

Alert if dns server does not respond:

    check_dns -H labs.consol.de
    OK - labs.consol.de returns 94.185.89.33 (A)

Check for specific type from specific server:

    check_dns -H consol.de -q MX -s 1.1.1.1
    OK - consol.de returns mail.consol.de. (MX)

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_dns
        use                  generic-service
        check_command        check_nrpe!check_dns!'-H' 'omd.consol.de'
    }

## Usage

```UNKNOWN - Usage:
  check_dns [OPTIONS]

Application Options:
  -H, --host=             The name or address you want to query
  -s, --server=           DNS server you want to use for the lookup
  -p, --port=             Port number you want to use (default: 53)
  -q, --querytype=        DNS record query type (default: A)
      --norec             Clears the Recursion Desired flag, DNS server answers only from its authoritative data or
                          cache, does not ask other nameservers.
  -e, --expected-string=  IP-ADDRESS string you expect the DNS server to return. If multiple IP-ADDRESS are returned at
                          once, you have to specify whole string
      --search-path=      Search paths to add to domains before sending a DNS query. This can be specified multiple
                          times.
      --resolv-conf-file= Path to the resolv.conf file to use. Is not used in Windows. (default: /etc/resolv.conf)
  -v, --verbose           Show verbose output.
  -w, --warning=          Return warning if elapsed time to get a successful DNS query exceeds this value in seconds.
                          Default is off.
  -c, --critical=         Return critical if elapsed time to get a successful DNS query exceeds this value in seconds.
                          Default ist off.
  -t, --timeout=          Exit early and return unknown if elapsed time to get a successful DNS query exceeds this
                          value in seconds. (default: 10)

Help Options:
  -h, --help              Show this help message
```
