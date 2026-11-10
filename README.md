# SNClient

<div>
  <img src="https://raw.githubusercontent.com/Consol-Monitoring/snclient/refs/heads/main/docs/logo/snclient_dark.svg?sanitize=true#gh-dark-mode-only" style="height: auto; width: 200px;">
  <img src="https://raw.githubusercontent.com/Consol-Monitoring/snclient/refs/heads/main/docs/logo/snclient.svg?sanitize=true#gh-light-mode-only" style="height: auto; width: 200px;">
</div>

[![CICD Pipeline](https://github.com/Consol-Monitoring/snclient/actions/workflows/builds.yml/badge.svg?branch=main)](https://github.com/Consol-Monitoring/snclient/actions/workflows/builds.yml)
[![Latest Release](https://img.shields.io/github/v/release/Consol-Monitoring/snclient?sort=semver)](https://github.com/Consol-Monitoring/snclient/releases)
[![Downloads](https://img.shields.io/github/downloads/Consol-Monitoring/snclient/total?color=yellow)](https://github.com/Consol-Monitoring/snclient/releases)
[![License](https://img.shields.io/github/license/Consol-Monitoring/snclient)](https://github.com/Consol-Monitoring/snclient/blob/main/LICENSE)
[![IRC](https://img.shields.io/badge/IRC-libera.chat%2F%23snclient-blue)](https://web.libera.chat/?nick=Guest?#snclient)

SNClient (Secure Naemon Client) is a general-purpose monitoring agent that replaces NRPE and NSClient++.

## Contact

* Report a security advisory through [GitHub Security](https://github.com/Consol-Monitoring/snclient/security).
* Join the mailing list on [Google Groups](https://groups.google.com/group/snclient).
* Ask a question on [Stack Overflow](https://stackoverflow.com/questions/tagged/snclient).
* Report a bug in [GitHub Issues](https://github.com/Consol-Monitoring/snclient/issues).
* Chat with developers on [IRC Libera #snclient](irc://irc.libera.chat/snclient) ([Webchat](https://web.libera.chat/?nick=Guest?#snclient)).
* Get professional support from [ConSol*](https://www.consol.com/product-solutions/open-source-monitoring/).

## Documentation

Read the documentation on [omd.consol.de](https://omd.consol.de/docs/snclient/).

The documentation source is in [docs/](docs/).

## Supported Operating Systems

|               | i386 | x86_64 | aarch64 (arm) |
|---------------|:----:|:------:|:-------------:|
| **Linux**     |   X  |    X   |   X           |
| **Windows**\* |   X  |    X   |   X           |
| **FreeBSD**   |   X  |    X   |   X           |
| **macOS**     |      |    X   |   X           |

\* Only Windows 10 / Windows Server 2016 or newer.

See the detailed list of [supported operating systems](docs/install/supported.md).

## Supported Protocols

* Prometheus HTTP(S)
* NRPE (v2/v4)
* NSCP REST API via HTTP(S) (checks only)

## Installation

Download prebuilt binaries and packages for all supported systems from the
[release page](https://github.com/Consol-Monitoring/snclient/releases).

Follow the [installation instructions](https://omd.consol.de/docs/snclient/install/).

## Check Plugin Status

|                                   | Windows |  Linux  |  macOS  |   BSD   |
|-----------------------------------|:-------:|:-------:|:-------:|:-------:|
| **check_alias**                   |    X    |    X    |    X    |    X    |
| **check_connections**             |    X    |    X    |    X    |    X    |
| **check_cpu_utilization**         |    X    |    X    |    X    |    X    |
| **check_cpu**                     |    X    |    X    |    X    |    X    |
| **check_dns**                     |    X    |    X    |    X    |    X    |
| **check_drivesize**               |    X    |    X    |    X    |    X    |
| **check_drive_io**                |    X    |    X    |    X    |    X    |
| **check_dummy**                   |    X    |    X    |    X    |    X    |
| **check_eventlog**                |    X    |         |         |         |
| **check_files**                   |    X    |    X    |    X    |    X    |
| **check_http**                    |    X    |    X    |    X    |    X    |
| **check_index**                   |    X    |    X    |    X    |    X    |
| **check_kernel_stats**            |         |    X    |         |         |
| **check_load**                    |    X    |    X    |    X    |    X    |
| **check_logfile**                 |    X    |    X    |    X    |    X    |
| **check_mailq**                   |         |    X    |    X    |    X    |
| **check_memory**                  |    X    |    X    |    X    |    X    |
| **check_mount**                   |    X    |    X    |    X    |    X    |
| **check_multi**                   |    X    |    X    |    X    |    X    |
| **check_network**                 |    X    |    X    |    X    |    X    |
| **check_nsc_web**                 |    X    |    X    |    X    |    X    |
| **check_ntp_offset**              |    X    |    X    |    X    |    X    |
| **check_omd**                     |         |    X    |         |         |
| **check_os_updates**              |    X    |    X    |    X    |         |
| **check_os_version**              |    X    |    X    |    X    |    X    |
| **check_pagefile**                |    X    |         |         |         |
| **check_pdh**                     |    X    |         |         |         |
| **check_ping**                    |    X    |    X    |    X    |    X    |
| **check_process**                 |    X    |    X    |    X    |    X    |
| **check_service**                 |    X    |    X    |         |         |
| **check_snclient**                |    X    |    X    |    X    |    X    |
| **check_snclient_version**        |    X    |    X    |    X    |    X    |
| **check_swap_io**                 |         |    X    |    X    |    X    |
| **check_tasksched**               |    X    |         |         |         |
| **check_tcp**                     |    X    |    X    |    X    |    X    |
| **check_temperature**             |         |    X    |         |         |
| **check_uptime**                  |    X    |    X    |    X    |    X    |
| **check_wmi**                     |    X    |         |         |         |
| **check_wrap / external scripts** |    X    |    X    |    X    |    X    |

## Roadmap

Planned changes:

* [ ] add time support in thresholds, e.g. warn=time > 18:00 && load > 10
* [ ] OpenTelemetry
* [ ] improve configuration
  * [ ] add a configuration validator
  * [ ] use strongly typed configuration items

## Not gonna happen

SNClient will most likely not include these features:

* CheckMK support
* Embedded Lua support
* Embedded Python support
* Graphite support
* NRDP support
* NSCA support
* SMTP support
* Website/REST API (except doing checks)
* check_nt support
