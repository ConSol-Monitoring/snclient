---
title: check_http
---

## check_http

Runs check_http to perform http(s) checks
It basically wraps the plugin from https://github.com/sni/check_http_go

- [Examples](#examples)
- [Usage](#usage)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

Alert if http server does not respond:

    check_http -H omd.consol.de
    HTTP OK - HTTP/1.1 200 OK - 573 bytes in 0.001 second response time | ...

Check for specific string and response code:

    check_http -H omd.consol.de -S -u "/docs/snclient/" -e 200,304 -s "consol" -vvv
    HTTP OK - Status line output "HTTP/2.0 200 OK" matched "200,304", Response body matched "consol"...

It can be a bit tricky to set the -u/--uri on windows, since the / is considered as start of
a command line parameter.

To avoid this issue, simply use the long form --uri=/path.. so the parameter does not start with a slash.

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_http
        use                  generic-service
        check_command        check_nrpe!check_http!'-H' 'omd.consol.de' '--uri=/docs' '-S'
    }

## Usage

```Usage:
  check_http [OPTIONS]

Application Options:
      --timeout=                  Timeout to wait for connection (default: 10s)
      --max-buffer-size=          Max buffer size to read response body
                                  (default: 1MB)
      --no-discard                raise error when the response body is larger
                                  then max-buffer-size
      --consecutive=              number of consecutive successful requests
                                  required (default: 1)
      --interim=                  interval time after successful request for
                                  consecutive mode (default: 1s)
      --wait-for                  retry until successful when enabled
      --wait-for-interval=        retry interval (default: 2s)
      --wait-for-max=             time to wait for success
  -H, --hostname=                 Host name using Host headers
  -I, --IP-address=               IP address or Host name
  -p, --port=                     Port number
  -j, --method=                   Set HTTP Method (default: GET)
  -u, --uri=                      URI to request (default: /)
  -e, --expect=                   Comma-delimited list of expected HTTP
                                  response status
  -s, --string=                   String to expect in the content
      --base64-string=            Base64 Encoded string to expect the content
  -A, --useragent=                UserAgent to be sent (default: check_http)
  -a, --authorization=            username:password on sites with basic
                                  authentication
  -S, --ssl                       use https
      --sni                       enable SNI
      --tls-max=[1.0|1.1|1.2|1.3] maximum supported TLS version
  -4                              use tcp4 only
  -6                              use tcp6 only
  -V, --version                   Show version
  -v, --verbose                   Show verbose output
      --proxy=                    Proxy that should be used

Help Options:
  -h, --help                      Show this help message
```
