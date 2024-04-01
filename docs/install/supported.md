---
linkTitle: Supported OS
---

# Supported Systems

We are trying to build the SNClient+ to be as compatible as possible. Due to
limited hardware, we are not able to test the agent on all operating systems.

If you are using SNClient+ anywhere not listed here, feel free to open an issue
on github to update this page.

If there are no installer package for your system available for download, you might still succeed
by [building snclient from source](build).

### CPU Architectures

|             | i386 | x64 | arm64     |
|-------------|:----:|:---:|:---------:|
| **Linux**   |   X  |  X  |   X       |
| **Windows** |   X  |  X  | (use x64) |
| **FreeBSD** |   X  |  X  |   X       |
| **MacOSX**  |      |  X  |   X       |

### Windows

Successfully tested on:

- Windows 8
- Windows 10
- Windows 11
- Windows Server 2019
- Windows ARM 11 Preview (using the x86 pkg)

Others will quite likely work as well, but haven't been tested yet.

### Linux

Debian:

- Debian >= 8
- Ubuntu >= 16.04

RedHat:

- RHEL >= 7

Others will quite likely work as well, but haven't been tested yet.

## Mac OSX / Darwin

- OSX >= 13

Others will quite likely work as well, but haven't been tested yet.

## FreeBSD

- FreeBSD >= 14

Others will quite likely work as well, but haven't been tested yet.

## Other

Feel free to open an issue on github if you are running the agent somewhere not
listed here.
