---
linkTitle: Building From Source
---

# Building SNClient From Source

## Requirements

- go >= 1.22
- openssl
- make
- help2man

## Building Binary

    %> git clone https://github.com/Consol-Monitoring/snclient
    %> cd snclient
    %> make dist
    %> make snclient

Certificates and a default .ini file will be in the `dist/` folder then and the
binary file is in the current folder.

### Building RPMs

Building RPM packages is available with the `make rpm` target:

    %> make rpm

You will need those extra requirements:

- rpm-build
- help2man
- (rpmlint)

### Debian/Ubuntu

Building Debian packages is available with the `make deb` target:

    %> make deb

You will need those extra requirements:

- dpkg
- help2man
- devscripts
- (lintian)

### OSX PKG

Building OSX packages is available with the `make osx` target:

    %> make osx

You will need those extra requirements:

- Xcode
- help2man (for ex. from homebrew)

### Windows MSI

Building a .msi installer package involves a couple of extra steps.

The package content will be prepared on a linux host and the final
msi package will then be build on windows host.

#### Prepare Windist Folder

First you need to prepare a `windist` folder. This can be done on any
linux machine by running `make windist`.

This are the requirements on your linux host:

- go >= 1.20
- openssl
- make

Then build the binary. For example for a 64bit binary run `make build-windows-amd64`
for a i386 32bit binary use `make build-windows-i386`

Then move the resulting .exe file into the windist folder and rename to `snclient.exe`

You now should have a folder looking like this:

    %> ls -la windist/
    -rw-r--r--  1 user group     2041 Jul 18 09:30 cacert.pem
    -rw-r--r--  1 user group     1456 Jul 18 09:30 server.crt
    -rw-------  1 user group     1704 Jul 18 09:30 server.key
    -rwxr-xr-x  1 user group 14193152 Jul 18 09:37 snclient.exe
    -rw-r--r--  1 user group     9394 Jul 18 09:30 snclient.ini

Get current version information, it will be required for the next step:

    %> ./buildtools/get_version
    0.04.0033

    %> git rev-parse --short HEAD
    5a30697

#### Build MSI

This are the requirements on your windows host:

- .net framework 3.5 ([download link](https://download.microsoft.com/download/0/6/1/061F001C-8752-4600-A198-53214C69B51F/dotnetfx35setup.exe))
- wix toolset (3.x) ([download link](https://github.com/wixtoolset/wix3/releases/download/wix3141rtm/wix314.exe))

Then build the msi like this using the version information (without leading zeros) from the previous step:

    & .\packaging\windows\build_msi.ps1 `
    -out "snclient.msi" `
    -arch "amd64" `
    -major "0" `
    -minor "4" `
    -rev "33" `
    -sha "5a30697"

The architecture can either be:

- `386` for 32bit i386 systems
- `amd64` for 64bit x86_64 systems
