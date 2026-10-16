---
linkTitle: Supported OS
---

# Supported Systems

We are trying to build the SNClient to be as compatible as possible. Due to
limited hardware, we are not able to test the agent on all operating systems.

If you are using SNClient anywhere not listed here, feel free to open an issue
on GitHub to update this page.

If no installer package is available for your system, you might still succeed
by [building SNClient from source](../build).

## CPU Architectures

|               | i386 | x86_64 | aarch64 (arm) |
|---------------|:----:|:------:|:-------------:|
| **Linux**     |   X  |    X   |   X           |
| **Windows**   |   X  |    X   |   X           |
| **FreeBSD**   |   X  |    X   |   X           |
| **macOS**     |      |    X   |   X           |

## Windows

Successfully tested on:

- Windows 10
- Windows 11
- Windows Server 2019
- Windows ARM 11 Preview
- It should work on any Windows newer than Windows 10 / Windows Server 2016.

The Go version required by SNClient (>= 1.27) does not support anything older than Windows 10 or Windows Server 2016.

## Linux

Debian:

- Debian >= 8
- Ubuntu >= 16.04

Red Hat:

- RHEL >= 7

Arch Linux:

- Current rolling release

Others will quite likely work as well, but haven't been tested yet.

## macOS / Darwin

- macOS >= 13

Others will quite likely work as well, but haven't been tested yet.

## FreeBSD

- FreeBSD >= 14

Others will quite likely work as well, but haven't been tested yet.

## Other

Feel free to open an issue on GitHub if you are running the agent somewhere not
listed here.
