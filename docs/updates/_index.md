---
title: Updates
linkTitle: Updates
weight: 500
---

## Manual Update

Manual updates usually follow the same steps as the initial installation. Download the latest release:

- [SNClient Releases Page](https://github.com/ConSol-Monitoring/snclient/releases)

Then install the file as described on the [install page](../install/).

## Automatic Updates

### SNClient Automatic Updates

If possible, use the operating system's update mechanism to install updates automatically.
Otherwise, SNClient can update itself periodically.

Configuration:

Create or edit `/etc/snclient/snclient_local.ini` (on Windows: `C:\Program Files\snclient\snclient_local.ini`)

```ini
[/settings/updates]
automatic updates = enabled
automatic restart = enabled
```

These settings update SNClient to the latest stable release.

To use development builds, for example on test systems, add the dev channel.

```ini
[/settings/updates]
automatic updates = enabled
automatic restart = enabled
channel = stable,dev

[/settings/updates/channel/dev]
; github token - the dev channel requires a github token to download the update
github token = GITHUB-TOKEN...
```

To use the dev channel, create a GitHub token at [github.com/settings/tokens](https://github.com/settings/tokens).

You cannot download the build artifacts without a token.

**Note:** SNClient self-update works on Linux, but cannot safely transfer the required capabilities.
It works fully only if you do not require capabilities.

### Debian / Ubuntu

On Debian and Ubuntu, use the `unattended-upgrades` package.

- [wiki.debian.org](https://wiki.debian.org/UnattendedUpgrades)

Install the package:

    #> apt install unattended-upgrades

After installation, add `labs.consol.de` to the list of sites for automatic updates:

/etc/apt/apt.conf.d/50unattended-upgrades

    Unattended-Upgrade::Origins-Pattern {
        // extend origin patterns with the labs repository
        "site=labs.consol.de"
    }

To activate periodic updates, open the root crontab:

    #> crontab -e

Add this entry to install updates at the start of every hour:

    0 * * * * /usr/bin/apt-get -qq update && /bin/bash -lc "/usr/bin/unattended-upgrade"

See [wiki.debian.org](https://wiki.debian.org/UnattendedUpgrades) for other methods.
