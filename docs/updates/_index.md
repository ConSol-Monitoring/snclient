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
verify signature = true
```

This will update SNClient to the latest stable release.

In case you want to use the development builds, ex. on test systems you can
add the dev channel as well.

```ini
[/settings/updates]
automatic updates = enabled
automatic restart = enabled
verify signature = true
channel = stable,dev

[/settings/updates/channel/dev]
; github token - the dev channel requires a github token to download the update
github token = GITHUB-TOKEN...
```

In order to use the dev channel, you need to create a github token here: [github.com/settings/tokens](https://github.com/settings/tokens)

Unfortunately it is not possible to download the build artifacts without a token.

Update signatures are verified by default with the public key built into
SNClient. Missing or invalid signatures abort the update. For manual recovery
from an unsigned source, verification can be explicitly disabled with
`verify signature = false` in `/settings/updates`.

The release workflow expects the same PEM-encoded ECDSA P-256 private key in both
GitHub environments:

- `SNCLIENT_UPDATE_PRIVATE_KEY_PEM` in the `update-stable` environment
- `SNCLIENT_UPDATE_PRIVATE_KEY_PEM` in the `update-dev` environment

The stable workflow signs the final RPM, MSI and PKG release assets. Development
signatures are created only for package artifacts built from a push to the
`main` branch. OpenSSL generates each binary `.sig` sidecar as an ASN.1/DER ECDSA
signature over the SHA-256 digest of the exact package bytes.

Generate the signing key once outside the repository:

```sh
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out update-private.pem
openssl pkey -in update-private.pem -pubout -out update-public.pem
```

Store the full private PEM as the secret in both environments. The client
stores the public key as base64-encoded PKIX DER in `update_signature.go`:

```sh
openssl pkey -in update-private.pem -pubout -outform DER | base64 -w0
```

The workflow also pins the SHA-256 fingerprint of the same DER public key and
fails before signing if the configured secret does not match:

```sh
openssl pkey -in update-private.pem -pubout -outform DER | sha256sum
```

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
