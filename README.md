# SNClient+
[![CICD Pipeline](https://github.com/sni/snclient/actions/workflows/cicd.yml/badge.svg?branch=main)](https://github.com/sni/snclient/actions/workflows/cicd.yml)
[![Latest Release](https://img.shields.io/github/v/release/sni/snclient?sort=semver)](https://github.com/sni/snclient/releases)
[![GitHub](https://img.shields.io/github/license/sni/snclient)](https://github.com/sni/snclient/blob/main/LICENSE)

SNClient+ is a secure general purpose monitoring agent designed as replacement for NRPE and NSClient++.

## Supported Operating Systems

|         | i386 | x64 | arm64 |
|---------|------|-----|-------|
| Linux   |   X  |  X  |   X   |
| Windows |   X  |  X  |       |
| FreeBSD |   X  |  X  |   X   |
| MacOSX  |      |  X  |   X   |

## Supported Protocols

 - Prometheus HTTP(s)
 - NRPE (v2)

## Installation
There are prebuild binaries and packages for the all supported systems (see above) on the
[release page](https://github.com/sni/snclient/releases).


Further details are covered in the [documentation](docs/install.md).

## Feature Comparison Table
soon...

## Roadmap

- [] check usr signal handler
- [] implement logging
- [] NRPE protocol support v3
- [] NRPE protocol support v4
- [] rework usage()
- [] add feature comparison to readme
- [] add docs/
- [] add support for includes in ini file
- [] add support for client certificates

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