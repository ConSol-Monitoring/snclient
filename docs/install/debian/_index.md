---
title: Debian / Ubuntu
linkTitle: Debian / Ubuntu
description: Install SNClient on Debian and Ubuntu using DEB packages.
weight: 300
---

# Debian / Ubuntu

## Installation

Stable release installation packages can be found here:

- [ConSol Software Repository](https://labs.consol.de/repo/stable/) (recommended)
- [GitHub Releases](https://github.com/ConSol-Monitoring/snclient/releases)

Add the repository as described on the linked repository page.

    #> apt install snclient

![apt install SNClient](apt.png "apt install snclient")

### Firewall

The firewall should be configured to allow these ports:

- `8443` : if you enabled the webserver (the default is enabled)
- `5666` : if you enabled the NRPE server (disabled by default)
- `9999` : if you enabled the Prometheus server (disabled by default)

## Uninstall

Uninstall SNClient using `apt`.

    #> apt remove snclient
