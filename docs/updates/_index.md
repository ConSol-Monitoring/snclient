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

If possible, use the operating system's own update mechanism to automatically install updates.
If not, SNClient can update itself periodically.

Configuration:

Create or edit `/etc/snclient/snclient_local.ini` (on Windows: `C:\Program Files\snclient\snclient_local.ini`)

```ini
[/settings/updates]
automatic updates = enabled
automatic restart = enabled
```

This will update SNClient to the latest stable release.

If you want to use development builds, for example on test systems, you can
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

In order to use the dev channel, you need to create a GitHub token here: [github.com/settings/tokens](https://github.com/settings/tokens)

Unfortunately it is not possible to download the build artifacts without a token.

**Note:** The SNClient self-update works on Linux, but cannot safely transfer the required capabilities. So it will
only fully work if you don't require any capabilities.

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

To install updates at the start of every hour.

Or see the [wiki.debian.org](https://wiki.debian.org/UnattendedUpgrades) for other ways.
