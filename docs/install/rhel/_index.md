---
title: Red Hat / Rocky / AlmaLinux
linkTitle: RHEL / Rocky / AlmaLinux
description: Install SNClient on RHEL, Rocky Linux, and AlmaLinux from packages.
weight: 200
---

# Red Hat / Rocky / AlmaLinux

## Installation

Stable release installation packages can be found here:

- [ConSol Software Repository](https://labs.consol.de/repo/stable/) (recommended)
- [GitHub Releases](https://github.com/ConSol-Monitoring/snclient/releases)

Add the repository as described on the linked repository page.

    #> dnf install snclient

### Firewall

The firewall should be configured to allow these ports:

- `8443` : if you enabled the webserver (the default is enabled)
- `5666` : if you enabled the NRPE server (disabled by default)
- `9999` : if you enabled the Prometheus server (disabled by default)

## Uninstall

Uninstall SNClient using `dnf`.

    #> dnf remove snclient
