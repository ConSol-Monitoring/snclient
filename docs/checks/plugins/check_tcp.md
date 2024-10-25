---
title: check_tcp
---

## check_tcp

Runs check_tcp to perform tcp connection checks.
It basically wraps the plugin from https://github.com/taku-k/go-check-plugins/tree/master/check-tcp

- [Examples](#examples)
- [Usage](#usage)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

Alert if tcp connection fails:

    check_tcp -H omd.consol.de -p 80
    TCP OK - 0.003 seconds response time on omd.consol.de port 80

Send something and expect specific string:

    check_tcp -H outlook.com -p 25 -s "HELO" -e "Microsoft ESMTP MAIL Service ready" -q "QUIT
    TCP OK - 0.197 seconds response time on outlook.com port 25

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
        service_description  check_tcp
        use                  generic-service
        check_command        check_nrpe!check_tcp!'-H' 'omd.consol.de' '-p' '80'
    }

## Usage

```Usage:
  check_tcp [OPTIONS]

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
  -E, --escape                Can use \n, \r, \t or \ in send or quit string. Must come before send or quit option. By default, nothing added to send, \r\n added to end of quit
  -W, --error-warning         Set the error level to warning when exiting with unexpected error (default: critical). In the case of request succeeded, evaluation result of -c option eval takes priority.
  -C, --expect-closed         Verify that the port/unixsock is closed. If the port/unixsock is closed, OK; if open, follow the ErrWarning flag. This option only verifies the connection.

Help Options:
  -h, --help                  Show this help message
```
