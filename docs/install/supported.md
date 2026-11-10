---
title: Supported Systems
linkTitle: Supported Systems
description: Review supported operating systems and CPU architectures for SNClient.
---

# Supported Systems

We aim to support as many systems as possible. Limited hardware prevents us from testing SNClient on all operating systems.

If you use SNClient on a system not listed here, open an issue on GitHub so we can update this page.

If no installer package is available for your system, you might still succeed
by [building SNClient from source](../build).

## CPU Architectures

|               | i386 | x86_64 | aarch64 (arm) | ppc64le |
|---------------|:----:|:------:|:-------------:|:-------:|
| **Linux**     |   X  |    X   |   X           |   X     |
| **Windows**   |   X  |    X   |   X           |         |
| **FreeBSD**   |   X  |    X   |   X           |         |
| **macOS**     |      |    X   |   X           |         |

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

Other systems will likely work, but we have not tested them yet.

## macOS / Darwin

- macOS >= 13

Other systems will likely work, but we have not tested them yet.

## FreeBSD

- FreeBSD >= 14

Other systems will likely work, but we have not tested them yet.

## Other

If you run the agent on a system not listed here, open an issue on GitHub.
