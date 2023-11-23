---
title: check_nsc_web
---

This builtin check command wraps the `check_nsc_web` plugin from https://github.com/ConSol-Monitoring/check_nsc_web

### Implementation

| Windows | Linux | FreeBSD | MacOSX |
|:-------:|:-----:|:-------:|:------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default check

    check_nsc_web -p ... -u https://localhost:8443
    OK: REST API reachable on https://localhost:8443

## Usage

    check_nsc_web -h

```
Usage:
  check_nsc_web [options] [query parameters]

Description:
  check_nsc_web is a REST client for the NSClient++/SNClient+ webserver for querying
  and receiving check information over HTTPS.

Version:
  check_nsc_web v0.6.1

Example:
  check_nsc_web -p "password" -u "https://<SERVER_RUNNING_NSCLIENT>:8443" check_cpu

  check_nsc_web -p "password" -u "https://<SERVER_RUNNING_NSCLIENT>:8443" check_drivesize disk=c

Options:
  -u <url>                 SNClient/NSCLient++ URL, for example https://10.1.2.3:8443
  -t <seconds>             Connection timeout in seconds. Default: 10
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
  -V                       Print program version
  -f <integer>             Round performance data float values to this number of digits. Default: -1
  -j                       Print out JSON response body
  -query <string>          Placeholder for query string from config file
```
