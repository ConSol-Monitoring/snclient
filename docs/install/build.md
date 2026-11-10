---
title: Building SNClient From Source
linkTitle: From Source
description: Build SNClient binaries and platform packages from source.
---

# Building SNClient From Source

## Requirements

- Go
- OpenSSL
- make
- help2man

## Building the Binary

    %> git clone https://github.com/Consol-Monitoring/snclient
    %> cd snclient
    %> make dist
    %> make snclient

The build puts certificates and a default .ini file in the `dist/` folder.
It puts the binary in the current folder.

### Building RPMs

Build RPM packages using the `make rpm` target:

    %> make rpm

You will need these additional dependencies:

- rpm-build
- help2man
- (rpmlint)

### Debian/Ubuntu

Build Debian packages using the `make deb` target:

    %> make deb

You will need these additional dependencies:

- dpkg
- help2man
- devscripts
- (lintian)

### macOS PKG

Build macOS packages using the `make osx` target:

    %> make osx

You will need these additional dependencies:

- Xcode
- help2man (for example, from Homebrew)

### Windows MSI

Prepare the .msi package contents on a Linux host. Then build the final MSI package on a Windows host.

#### Prepare Windist Folder

Run `make windist` on any Linux machine to prepare a `windist` folder.

These are the requirements on your Linux host:

- Go
- OpenSSL
- make

Then build the binary. For a 64-bit binary, run `make build-windows-amd64`.
For an i386 32-bit binary, use `make build-windows-i386`.

Then move the resulting .exe file into the windist folder and rename it to `snclient.exe`.

The folder should now look like this:

    %> ls -la windist/
    -rw-r--r--  1 user group     2041 Jul 18 09:30 cacert.pem
    -rw-r--r--  1 user group     1456 Jul 18 09:30 server.crt
    -rw-------  1 user group     1704 Jul 18 09:30 server.key
    -rwxr-xr-x  1 user group 14193152 Jul 18 09:37 snclient.exe
    -rw-r--r--  1 user group     9394 Jul 18 09:30 snclient.ini

Get the current version information for the next step:

    %> ./buildtools/get_version
    0.04.0033

    %> git rev-parse --short HEAD
    5a30697

#### Build MSI

These are the requirements on your Windows host:

- .NET Framework 3.5 ([download link](https://download.microsoft.com/download/0/6/1/061F001C-8752-4600-A198-53214C69B51F/dotnetfx35setup.exe))
- WiX Toolset (3.x) ([download link](https://github.com/wixtoolset/wix3/releases/download/wix3141rtm/wix314.exe))

Build the MSI package with the version information from the previous step. Remove leading zeros from the version numbers:

    & .\packaging\windows\build_msi.ps1 `
    -out "snclient.msi" `
    -arch "amd64" `
    -major "0" `
    -minor "4" `
    -rev "33" `
    -sha "5a30697"

The architecture can be either:

- `386` for 32-bit i386 systems
- `amd64` for 64-bit x86_64 systems
