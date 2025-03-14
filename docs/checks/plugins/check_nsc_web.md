---
title: check_nsc_web
---

## check_nsc_web

Runs check_nsc_web to perform checks on other snclient agents.
It basically wraps the plugin from https://github.com/ConSol-Monitoring/check_nsc_web

- [Examples](#examples)
- [Usage](#usage)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

    check_nsc_web -p ... -u https://localhost:8443
    OK - REST API reachable on https://localhost:8443

Check specific plugin:

    check_nsc_web -p ... -u https://localhost:8443 -c check_process process=snclient.exe
    OK - all 1 processes are ok. | ...

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_nsc_web
        use                  generic-service
        check_command        check_nrpe!check_nsc_web!'-H' 'omd.consol.de' '--uri=/docs' '-S'
    }

## Usage

```Usage:
  check_nsc_web [options] [query parameters]

Description:
  check_nsc_web is a REST client for the NSClient++/SNClient webserver for querying
  and receiving check information over HTTP(S).

Version:
  check_nsc_web v0.7.2

Example:
  connectivity check (parent service):
  check_nsc_web -p "password" -u "https://<SERVER>:8443"

  check without arguments:
  check_nsc_web -p "password" -u "https://<SERVER>:8443" check_cpu

  check with arguments:
  check_nsc_web -p "password" -u "https://<SERVER>:8443" check_drivesize disk=c

Options:
  -u <url>                 SNClient/NSCLient++ URL, for example https://10.1.2.3:8443
  -t <seconds>[:<STATE>]   Connection timeout in seconds. Default: 10sec
                           Optional set timeout state: 0-3 or OK, WARNING, CRITICAL, UNKNOWN
                           (default timeout state is UNKNOWN)
  -e <STATE>               exit code for connection errors. Default is UNKNOWN.
  -a <api version>         API version of SNClient/NSClient++ (legacy or 1) Default: legacy
  -l <username>            REST webserver login. Default: admin
  -p <password>            REST webserver password
  -config <file>           Path to config file

TLS/SSL Options:
  -C <pem file>            Use client certificate (pem) to connect. Must provide -K as well
  -K <key file>            Use client certificate key file to connect
  -ca <pem file>           Use certificate ca to verify server certificate
  -tlsmax <string>         Maximum tls version used to connect
  -tlsmin <string>         Minimum tls version used to connect. Default: tls1.0
  -tlshostname <string>    Use this servername when verifying tls server name
  -k                       Insecure mode - skip TLS verification

Output Options:
  -h                       Print help
  -v                       Enable verbose output
  -vv                      Enable very verbose output (and log directly to stdout)
  -V                       Print program version
  -f <integer>             Round performance data float values to this number of digits. Default: -1
  -j                       Print out JSON response body
  -r                       Print raw result without pre/post processing
  -query <string>          Placeholder for query string from config file
```
