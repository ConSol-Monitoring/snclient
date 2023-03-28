# SNClient+
[![CICD Pipeline](https://github.com/sni/snclient/actions/workflows/cicd.yml/badge.svg?branch=main)](https://github.com/sni/snclient/actions/workflows/cicd.yml)

SNClient+ is a general purpose monitoring agent designed as replacement for NRPE and NSClient++.

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

## Feature Comparison Table
soon...

## Roadmap

	- [] check usr signal handler
	- [] implement logging
	- [] NRPE protocol support v3
	- [] NRPE protocol support v4
	- [] add general tls support
	- [] rework usage()
	- [] add feature comparison to readme
	- [] add docs/
	- [] add support for variables in ini file