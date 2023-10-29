---
title: check_http
---

This builtin check command wraps the `check_dns` plugin from https://github.com/mackerelio/go-check-plugins/tree/master/check-dns

### Implementation

| Windows | Linux | FreeBSD | MacOSX |
|:-------:|:-----:|:-------:|:------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default check

    check_dns -H labs.consol.de
    OK: labs.consol.de returns 94.185.89.33 (A)

## Usage

    check_dns -h

    Application Options:
      -H, --host=            The name or address you want to query
      -s, --server=          DNS server you want to use for the lookup
      -p, --port=            Port number you want to use (default: 53)
      -q, --querytype=       DNS record query type (default: A)
          --norec            Set not recursive mode
      -e, --expected-string= IP-ADDRESS string you expect the DNS server to return. If multiple IP-ADDRESS are returned at once, you have to specify whole string

    Help Options:
      -h, --help             Show this help message