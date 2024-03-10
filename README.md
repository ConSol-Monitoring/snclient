# SNClient+

[![CICD Pipeline](https://github.com/Consol-Monitoring/snclient/actions/workflows/builds.yml/badge.svg?branch=main)](https://github.com/Consol-Monitoring/snclient/actions/workflows/builds.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Consol-Monitoring/snclient)](https://goreportcard.com/report/github.com/Consol-Monitoring/snclient)
[![Latest Release](https://img.shields.io/github/v/release/Consol-Monitoring/snclient?sort=semver)](https://github.com/Consol-Monitoring/snclient/releases)
[![License](https://img.shields.io/github/license/Consol-Monitoring/snclient)](https://github.com/Consol-Monitoring/snclient/blob/main/LICENSE)
[![IRC](https://img.shields.io/badge/IRC-libera.chat%2F%23snclient-blue)](https://web.libera.chat/?nick=Guest?#snclient)
<a href="https://omd.consol.de/docs/snclient/logo/"><img src="./docs/logo/snclient.svg" style="float:right; margin: 3px; height: auto; width: 200px; float: right;"></a>

SNClient+ (Secure Naemon Client) is a general purpose monitoring agent designed as replacement for NRPE and NSClient++.

## Contact

* Report a Security Advisory via [GitHub Security](https://github.com/Consol-Monitoring/snclient/security).
* Mailing list on [Google Groups](https://groups.google.com/group/snclient).
* Ask a question on [Stack Overflow](https://stackoverflow.com/questions/tagged/snclient)
* File a Bug in [GitHub Issues](https://github.com/Consol-Monitoring/snclient/issues).
* Chat with developers on [IRC Libera #snclient](irc://irc.libera.chat/snclient) ([Webchat](https://web.libera.chat/?nick=Guest?#snclient)).
* Professional Support from [ConSol*](https://www.consol.de/product-solutions/open-source-monitoring/)

## Documentation

All documentation is under `docs/`

## Supported Operating Systems

|             | i386 | x64 | arm64     |
|-------------|:----:|:---:|:---------:|
| **Linux**   |   X  |  X  |   X       |
| **Windows** |   X  |  X  | (use x64) |
| **FreeBSD** |   X  |  X  |   X       |
| **MacOSX**  |      |  X  |   X       |

A more detailed list of [supported operating systems](https://omd.consol.de/docs/snclient/install/supported/).

## Supported Protocols

* Prometheus HTTP(s)
* NRPE (v2/v4)
* NSCP Rest API via HTTP(s) (checks only)

## Installation

There are pre-build binaries and packages for the all supported systems (see above) on the
[release page](https://github.com/Consol-Monitoring/snclient/releases).

Further details are covered in the [documentation](https://omd.consol.de/docs/snclient/install/).

## Check Plugin Status

|                                   | Windows |  Linux  |   OSX   |   BSD   |
|-----------------------------------|:-------:|:-------:|:-------:|:-------:|
| **check_alias**                   |    X    |    X    |    X    |    X    |
| **check_connections**             |    X    |    X    |    X    |    X    |
| **check_cpu_utilization**         |    X    |    X    |    X    |    X    |
| **check_cpu**                     |    X    |    X    |    X    |    X    |
| **check_dns**                     |    X    |    X    |    X    |    X    |
| **check_drivesize**               |    X    |    X    |    X    |    X    |
| **check_dummy**                   |    X    |    X    |    X    |    X    |
| **check_eventlog**                |    X    |         |         |         |
| **check_files**                   |    X    |    X    |    X    |    X    |
| **check_http**                    |    X    |    X    |    X    |    X    |
| **check_index**                   |    X    |    X    |    X    |    X    |
| **check_kernel_stats**            |         |    X    |         |         |
| **check_load**                    |    X    |    X    |    X    |    X    |
| **check_mailq**                   |         |    X    |    X    |    X    |
| **check_memory**                  |    X    |    X    |    X    |    X    |
| **check_mount**                   |         |    X    |         |         |
| **check_network**                 |    X    |    X    |    X    |    X    |
| **check_nsc_web**                 |    X    |    X    |    X    |    X    |
| **check_ntp_offset**              |    X    |    X    |    X    |    X    |
| **check_omd**                     |         |    X    |         |         |
| **check_os_updates**              |    X    |    X    |    X    |         |
| **check_os_version**              |    X    |    X    |    X    |    X    |
| **check_pagefile**                |    X    |         |         |         |
| **check_ping**                    |    X    |    X    |    X    |    X    |
| **check_process**                 |    X    |    X    |    X    |    X    |
| **check_service**                 |    X    |    X    |         |         |
| **check_snclient_version**        |    X    |    X    |    X    |    X    |
| **check_tasksched**               |    X    |         |         |         |
| **check_tcp**                     |    X    |    X    |    X    |    X    |
| **check_temperature**             |         |    X    |         |         |
| **check_uptime**                  |    X    |    X    |    X    |    X    |
| **check_wmi**                     |    X    |         |         |         |
| **check_wrap / external scripts** |    X    |    X    |    X    |    X    |

## Roadmap

Find a brief overview of what is planned and what is done already:

### Stage 1

* [X] support NRPE clients
* [X] support NSCP rest api clients
* [X] support basic Prometheus metrics
* [X] implement reading nsclient.ini files
* [X] implement ssl/tls support
* [X] implement authenticaton / authorization
  * [X] basic auth
  * [X] client certificates
  * [X] allowed hosts
  * [X] allow arguments
  * [X] allow nasty characters
* [X] add build pipeline
  * [X] build windows msi packages
  * [X] build debian/ubuntu .deb packages
  * [X] build rhel/sles .rpm packages
  * [X] build osx .pkg packages
* [X] implement log rotation for file logger
* [X] self update (from configurable url)
* [X] implement perf-config
* [ ] finish builtin checks
* [X] implement help with examples and filters
* [X] review check plugin status

### Stage 2

* [X] add basic prometheus exporters
  * [X] exporter_exporter
  * [X] windows_exporter
  * [X] node_exporter
  * [ ] add time support in threshold, ex.: warn=time > 18:00 && load > 10
* [X] add config include folder
* [X] add check_ping plugin
* [X] add ntp check
* [ ] check usr signal handler
* [X] manage certificate via rest api

### Stage 3

* [X] self update from github
* [ ] open telemetry
* [ ] improve configuration
  * [ ] add config validator
  * [ ] use strong typed config items
* [ ] osx
  * [ ] check pkg uninstall
* [ ] rename packages to avoid confusion: amd64 -> x86-64, 386 -> i386, amd64 -> aarch64

## Not gonna happen

The following things will most likely not be part of snclient any time:

* CheckMK support
* Embedded LUA support
* Embedded Python support
* Graphite support
* NRDP support
* NSCA support
* SMTP support
* Website/Rest API (except doing checks)
* check_nt support
