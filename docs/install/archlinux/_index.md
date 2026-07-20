---
linkTitle: Arch Linux
weight: 350
---

# Arch Linux

## Installation

SNClient is available from the Arch User Repository (AUR) in three variants:

- [`snclient`](https://aur.archlinux.org/packages/snclient) builds the latest stable
  release from source. The instructions below use this package.
- [`snclient-bin`](https://aur.archlinux.org/packages/snclient-bin) uses pre-built
  release binaries and avoids compiling SNClient locally.
- [`snclient-git`](https://aur.archlinux.org/packages/snclient-git) builds the
  latest development revision and is intended for testing current changes.

Packages in the AUR are community-created user content. They are unofficial and
have not been thoroughly vetted. Manually verify the `PKGBUILD` and all other
files in the package repository before building or installing them. Refer to the
[Arch User Repository documentation](https://wiki.archlinux.org/title/Arch_User_Repository)
for general installation instructions and security guidance.

Install `git` and the `base-devel` package, then clone the package repository:

    #> pacman -S --needed git base-devel
    %> git clone https://aur.archlinux.org/snclient.git
    %> cd snclient

Review the package files and, as a regular user, build and install SNClient:

    %> makepkg -si

### Firewall

The firewall should be configured to allow these ports:

- `8443` : if you enabled the webserver (the default is enabled)
- `5666` : if you enabled the NRPE server (disabled by default)
- `9999` : if you enabled the Prometheus server (disabled by default)

## Uninstall

Uninstall is available with `pacman`.

    #> pacman -R snclient
