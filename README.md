# SNClient+
[![CICD Pipeline](https://github.com/Consol-Monitoring/snclient/actions/workflows/cicd.yml/badge.svg?branch=main)](https://github.com/Consol-Monitoring/snclient/actions/workflows/cicd.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Consol-Monitoring/snclient)](https://goreportcard.com/report/github.com/Consol-Monitoring/snclient)
[![Latest Release](https://img.shields.io/github/v/release/Consol-Monitoring/snclient?sort=semver)](https://github.com/Consol-Monitoring/snclient/releases)
[![License](https://img.shields.io/github/license/Consol-Monitoring/snclient)](https://github.com/Consol-Monitoring/snclient/blob/main/LICENSE)
[![IRC](https://img.shields.io/badge/IRC-libera.chat%2F%23snclient-blue)](https://web.libera.chat/?nick=Guest?#snclient)

SNClient+ (Secure Naemon Client) is a secure general purpose monitoring agent designed as replacement for NRPE and NSClient++.

## Supported Operating Systems

|         | i386 | x64 | arm64 |
|---------|------|-----|-------|
| Linux   |   X  |  X  |   X   |
| Windows |   X  |  X  |       |
| FreeBSD |   X  |  X  |   X   |
| MacOSX  |      |  X  |   X   |

## Supported Protocols

 - Prometheus HTTP(s)
 - NRPE (v2/v4)
 - NSCP Rest API via HTTP(s) (checks only)

## Installation
There are prebuild binaries and packages for the all supported systems (see above) on the
[release page](https://github.com/Consol-Monitoring/snclient/releases).


Further details are covered in the [documentation](docs/install.md).

## Implementation Status
W: work in progress
X: completed

|                        | Windows |  Linux  |   OSX   |   BSD   |
|------------------------|---------|---------|---------|---------|
| check_alias            |    X    |    X    |    X    |    X    |
| check_cpu              |    X    |    X    |    X    |    X    |
| check_drivesize        |    X    |    W    |    W    |    W    |
| check_dummy            |    X    |    X    |    X    |    X    |
| check_eventlog         |    W    |         |         |         |
| check_files            |    W    |    W    |    W    |    W    |
| check_index            |    X    |    X    |    X    |    X    |
| check_memory           |    X    |    X    |    X    |    X    |
| check_network          |    W    |    W    |    W    |    W    |
| check_os_version       |    X    |    X    |    X    |    X    |
| check_process          |    W    |    W    |    W    |    W    |
| check_service          |    X    |         |         |         |
| check_snclient_version |    X    |         |         |         |
| check_uptime           |    X    |    X    |    X    |    X    |
| check_wmi              |    W    |         |         |         |
| check_wrap             |    W    |    W    |    W    |    W    |


## Roadmap
Find a brief overview of what is planned and what is done already:

### Stage 1
- [X] support NRPE clients
- [X] support NSCP rest api clients
- [X] support basic Prometheus metrics
- [X] implement internal checks
  - [ ] CheckExternalScripts
  - [ ] check_drivesize
  - [ ] check_files
  - [ ] check_eventlog
  - [ ] check_cpu
  - [ ] check_memory
  - [ ] check_os_version
  - [ ] check_uptime
  - [ ] check_network
  - [ ] check_process
  - [ ] check_service
  - [ ] check_wmi
- [X] implement reading nsclient.ini files
- [X] implement ssl/tls support
- [X] implement authenticaton / authorization
  - [X] basic auth
  - [ ] client certificates
  - [X] allowed hosts
  - [X] allow arguments
  - [X] allow nasty characters
- [X] add build pipeline
  - [X] build windows msi packages
  - [X] build debian/ubuntu .deb packages
  - [X] build rhel/sles .rpm packages
  - [X] build osx .pkg packages
- [X] implement log rotation for file logger
- [X] self update (from configurable url)

### Stage 2
- [ ] add basic prometheus exporters
  - [ ] exporter_exporter
  - [ ] windows_exporter
  - [ ] node_exporter
  - [ ] add time support in threshold, ex.: warn=time > 18:00 && load > 10

### Stage 3
- [ ] add basic prometheus exporters
- [ ] self update from github

### Miscellaneous
- [ ] check usr signal handler
- [ ] Rest API
  - [ ] add performance data support
- [ ] improve configuration
  - [ ] add config validator
  - [ ] use strong typed config items
- [ ] improve documentation
  - [ ] add feature comparison to readme
  - [ ] add docs/
- [ ] osx
  - [ ] check pkg uninstall
- [ ] rename packages to avoid confusion: amd64 -> x86-64, 386 -> i386, amd64 -> aarch64

## Not gonna happen
The following things will most likely not be part of snclient any time:

- CheckMK support
- Embedded LUA support
- Embedded Python support
- Graphite support
- NRDP support
- NSCA support
- SMTP support
- Website/Rest API (except doing checks)
