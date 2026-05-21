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
  -H, --hostname=                                                 Host name using Host headers
  -I, --IP-address=                                               IP address or Host name
  -j, --method=                                                   Set HTTP Method (default: GET)
  -u, --uri=                                                      URI to request (default: /)
  -e, --expect=                                                   Comma-delimited list of expected HTTP response
                                                                  status. By default, 1XX, 2XX are OK, 3XX depends on
                                                                  --onredirect option, 4XX are WARNING, 5XX are CRITICAL
  -s, --string=                                                   String to expect in the content
      --base64-string=                                            Base64 Encoded string to expect the content
  -A, --useragent=                                                UserAgent to be sent (default: check_http)
  -a, --authorization=                                            username:password on sites with basic authentication
  -C, --certificate=                                              check certificates instead of content. Specified in
                                                                  mandatory days left to warn and optional days to crit
                                                                  with a comma: warn_days[,<crit_days>]
      --tls-min=[1.0|1.0+|1.1|1.1+|1.2|1.2+|1.3]                  minimum supported TLS version. Values with plus set
                                                                  the max tls version as well to latest version: 1.3
      --tls-max=[1.0|1.1|1.2|1.3]                                 maximum supported TLS version
      --proxy=                                                    Proxy that should be used
  -r, --regex=                                                    Search page for case-sensitive regex string
  -R, --regexi=                                                   Search page for case-insensitive regex string
  -f, --onredirect=[ok|warning|critical|follow|sticky|stickyport] What strategy to use when encountering a redirect.
                                                                  ok/warning/critical returns immediately. follow uses
                                                                  the new URL returned by golang HTTP client. Sticky
                                                                  keeps the hostname to be same after redirect, and
                                                                  stickyport persists the port as well.
      --max-buffer-size=                                          Max buffer size to read response body (default: 1MB)
  -t, --timeout=                                                  Timeout to wait for connection. If no time unit is
                                                                  given at the end, default of seconds is assumed
                                                                  (default: 10)
  -w, --warning=                                                  If the request+response takes longer specified
                                                                  warning threshold, raises a warning. If no time unit
                                                                  is given at the end, default of seconds is assumed.
                                                                  Value is truncated to milliseconds. (default: 30)
  -c, --critical=                                                 If the request+response takes longer specified
                                                                  critical threshold, raises a critical. If no time
                                                                  unit is given at the end, default of seconds is
                                                                  assumed. Value is truncated to milliseconds.
                                                                  (default: 60)
      --wait-for-interval=                                        retry interval (default: 2s)
      --wait-for-max=                                             time to wait for success
      --interim=                                                  interval time after successful request for
                                                                  consecutive mode (default: 1s)
      --consecutive=                                              number of consecutive successful requests required
                                                                  (default: 1)
  -p, --port=                                                     Port number
      --max-redirs=                                               Maximum redirects before giving up on following
      --no-discard                                                raise error when the response body is larger then
                                                                  max-buffer-size
      --wait-for                                                  retry until successful when enabled
  -S, --ssl                                                       use https
      --sni                                                       enable SNI
  -4                                                              use tcp4 only
  -6                                                              use tcp6 only
  -V, --version                                                   Show version
  -v, --verbose                                                   Show verbose output
      --show-body                                                 Print body content below status line
      --ignore-certificate-chain                                  by default all certificates are checked in many
                                                                  aspects. Toggle this option to only check the leaf
                                                                  (final) certificate.
      --dont-ignore-host-cn                                       Certificate subject's Common Name should matches the
                                                                  hostname. Common Name field is now largely unused in
                                                                  modern web, with Subject Alternative Name fields
                                                                  taking their place when present. It is ignored by
                                                                  default, use this flag to enable it.
      --ignore-san                                                Skip checking Subject Alternative Names against the
                                                                  hostname. SANs contain the hostnames and IP addresses
                                                                  this certificate is valid for.
      --ignore-not-after                                          Certificates are invalid after the timestamp in their
                                                                  NotAfter has passed. This field can be ignored with
                                                                  this flag.
      --ignore-not-before                                         Certificates are invalid before the timestamp in
                                                                  their NotBefore is reached. This field can be ignored
                                                                  with this flag.
      --ignore-signature-algorithm                                Some signature algorithms are deemed insecure, and
                                                                  are deprecated. The algorithm used can be ignored
                                                                  with this flag.

Help Options:
  -h, --help                                                      Show this help message
```
