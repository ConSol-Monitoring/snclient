---
title: Updates
linkTitle: Updates
weight: 500
---

## Manual Update

Manual updates usually work the same way as the initial installation. So you
simply download the latest release:

- [SNClient Releases Page](https://github.com/ConSol-Monitoring/snclient/releases)

Then install the file as described on the [install page](../install/).

## Automatic Updates

### SNClient Automatic Updates

If possible, use the OS own solution to automatically install updates.
If not, SNClient can install periodic update itself.

Configuration:

Create or edit `/etc/snclient/snclient_local.ini` (on windows: `C:\Program Files\snclient\snclient_local.ini`)

```ini
[/settings/updates]
automatic updates = enabled
automatic restart = enabled
```

This will update SNClient to the latest stable release.

In case you want to use the development builds, ex. on test systems you can
add the dev channel as well.

```ini
[/settings/updates]
automatic updates = enabled
automatic restart = enabled
channel = stable,dev

[/settings/updates/channel/dev]
; github token - the dev channel requires a github token to download the update
github token = GITHUB-TOKEN...
```

In order to use the dev channel, you need to create a github token here: [github.com/settings/tokens](https://github.com/settings/tokens)

Unfortunately it is not possible to download the build artifacts without a token.

### Debian / Ubuntu

On Debian and Ubuntu you can make use of the `unattended-upgrades` package.

- [wiki.debian.org](https://wiki.debian.org/UnattendedUpgrades)

Install the package:

    #> apt install unattended-upgrades

Once installed you can add `labs.consol.de` to the list of sites used
for automatic updates:

/etc/apt/apt.conf.d/50unattended-upgrades

    Unattended-Upgrade::Origins-Pattern {
        // extend origin patterns with the labs repository
        "site=labs.consol.de"
    }

Then activate periodic updates by adding a cronjob entry to the root crontab:

    #> crontab -e

And add:

    0 * * * * /usr/bin/apt-get -qq update && /bin/bash -lc "/usr/bin/unattended-upgrade"

To install updates every full hour.

Or see the [wiki.debian.org](https://wiki.debian.org/UnattendedUpgrades) for other ways.
