---
title: check_ssh
---

## check_ssh

Runs check_tcp with an SSH configururation to check for a running SSH server.
It basically wraps the plugin from https://github.com/taku-k/go-check-plugins/tree/master/check-tcp

- [Examples](#examples)
- [Usage](#usage)

## Implementation

| Windows            | Linux              | FreeBSD            | MacOSX             |
|:------------------:|:------------------:|:------------------:|:------------------:|
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### Default Check

		check_ssh github.com
SSH OK - 0.234 seconds response time on github.com port 22 [SSH-2.0-8ad108e] | time=0.234029s;;;0.000000;10.000000

		check_ssh --hostname github.com --warning 1
SSH OK - 0.262 seconds response time on github.com port 22 [SSH-2.0-8ad108e] | time=0.262048s;;;1.000000;10.000000

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_ssh
        use                  generic-service
        check_command        check_nrpe!check_ssh!'-H' '192.168.178.100' '-p' '2323'
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
  -c, --critical=             Response time to result in critical status (seconds) (default: 10)
  -E, --escape                Can use \n, \r, \t or \ in send or quit string. Must come before send or quit option. By
                              default, nothing added to send, \r\n added to end of quit
  -W, --error-warning         Set the error level to warning when exiting with unexpected error (default: critical). In
                              the case of request succeeded, evaluation result of -c option eval takes priority.
  -C, --expect-closed         Verify that the port/unixsock is closed. If the port/unixsock is closed, OK; if open,
                              follow the ErrWarning flag. This option only verifies the connection.
  -v, --verbose               Enables verbose logging of the actions taken.

Help Options:
  -h, --help                  Show this help message
```
