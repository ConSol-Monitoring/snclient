---
title: check_tcp
---

This builtin check command wraps the `check_tcp` plugin from https://github.com/taku-k/go-check-plugins/tree/master/check-tcp

### Implementation

| Windows | Linux | FreeBSD | MacOSX |
|:-------:|:-----:|:-------:|:------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default check

    check_tcp -H omd.consol.de
    TCP OK - 0.095 seconds response time on omd.consol.de port 80

## Usage

    Application Options:
          --service=              Service name. e.g. ftp, smtp, pop, imap and so on
      -H, --hostname=             Host name or IP Address
      -p, --port=                 Port number
      -s, --send=                 String to send to the server
      -e, --expect-pattern=       Regexp pattern to expect in server response
      -q, --quit=                 String to send server to initiate a clean close of the connection
      -S, --ssl                   Use SSL for the connection.
      -U, --unix-sock=            Unix Domain Socket
          --no-check-certificate  Do not check certificate
      -t, --timeout=              Seconds before connection times out (default: 10)
      -m, --maxbytes=             Close connection once more than this number of bytes are received
      -d, --delay=                Seconds to wait between sending string and polling for response
      -w, --warning=              Response time to result in warning status (seconds)
      -c, --critical=             Response time to result in critical status (seconds)
      -E, --escape                Can use \n, \r, \t or \ in send or quit string. Must come before send or quit option. By default, nothing added to send, \r\n
                                  added to end of quit
      -W, --error-warning         Set the error level to warning when exiting with unexpected error (default: critical). In the case of request succeeded,
                                  evaluation result of -c option eval takes priority.
      -C, --expect-closed         Verify that the port/unixsock is closed. If the port/unixsock is closed, OK; if open, follow the ErrWarning flag. This option
                                  only verifies the connection.

    Help Options:
      -h, --help                  Show this help message
