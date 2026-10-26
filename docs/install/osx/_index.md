---
title: macOS
linkTitle: macOS
description: Install SNClient on macOS using the package installer.
weight: 400
---

# macOS

## Installation

Stable release installation packages can be found here:

- [GitHub Releases](https://github.com/ConSol-Monitoring/snclient/releases)

Download the .pkg file from the releases page and start it. You might need to
open System Settings and allow opening the package since it is not signed.

![system settings](osx-settings.png "osx system settings")

This should open the installer:

![pkg installer](osx-pkg.png "osx pkg installer")

### Firewall

The firewall should be configured to allow these ports:

- `8443` : if you enabled the webserver (the default is enabled)
- `5666` : if you enabled the NRPE server (disabled by default)
- `9999` : if you enabled the Prometheus server (disabled by default)

## Uninstall

Uninstall SNClient using the uninstall shell script.

    %> sudo /usr/local/bin/snclient_uninstall.sh
