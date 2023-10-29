---
title: check_http
---

This builtin check command wraps the `check_http` plugin from https://github.com/sni/check_http_go

### Implementation

| Windows | Linux | FreeBSD | MacOSX |
|:-------:|:-----:|:-------:|:------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default check

    check_http -H omd.consol.de
    HTTP OK: HTTP/1.1 200 OK - 573 bytes in 0.001 second response time

## Usage

    check_http -h

    Usage:
      check_http [options]

    Application Options:
          --timeout=                  Timeout to wait for connection (default: 10s)
          --max-buffer-size=          Max buffer size to read response body (default: 1MB)
          --no-discard                raise error when the response body is larger then max-buffer-size
          --consecutive=              number of consecutive successful requests required (default: 1)
          --interim=                  interval time after successful request for consecutive mode (default: 1s)
          --wait-for                  retry until successful when enabled
          --wait-for-interval=        retry interval (default: 2s)
          --wait-for-max=             time to wait for success
      -H, --hostname=                 Host name using Host headers
      -I, --IP-address=               IP address or Host name
      -p, --port=                     Port number
      -j, --method=                   Set HTTP Method (default: GET)
      -u, --uri=                      URI to request (default: /)
      -e, --expect=                   Comma-delimited list of expected HTTP response status (default: HTTP/1.,HTTP/2.)
      -s, --string=                   String to expect in the content
          --base64-string=            Base64 Encoded string to expect the content
      -A, --useragent=                UserAgent to be sent (default: check_http)
      -a, --authorization=            username:password on sites with basic authentication
      -S, --ssl                       use https
          --sni                       enable SNI
          --tls-max=[1.0|1.1|1.2|1.3] maximum supported TLS version
      -4                              use tcp4 only
      -6                              use tcp6 only
      -V, --version                   Show version
      -v, --verbose                   Show verbose output

    Help Options:
      -h, --help                      Show this help message
