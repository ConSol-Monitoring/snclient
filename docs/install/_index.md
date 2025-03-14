---
title: Installation
linkTitle: Installation
weight: 400
---

## Installation

There is a OS specific installation documentation available for:

- [Windows](windows)
- [RHEL/Rocky/Alma](rhel)
- [Debian/Ubuntu](debian)
- [Mac OSX](osx)

## Using Binary Packages

Using packages is the recommended way to install SNClient.

Find a list of [supported OS distributions](supported) here.

### Stable Releases

Stable release installation packages can be found here:

- [SNClient Releases Page](https://github.com/ConSol-Monitoring/snclient/releases)

You might need to expand the assets to find the download links.
![Assets](download.png "Expand Assets")

### Development Snapshots

During development each code commit produces build artifacts if all tests were
successful.

Usually you should stick to the stable releases unless told otherwise or you want
to test something.

Installing the development snapshot is straight forward:

1. Open the [SNClient Github Actions](https://github.com/ConSol-Monitoring/snclient/actions/workflows/builds.yml?query=branch%3Amain) page
2. Choose first green build like in
   ![Actions](actions.png "Choose latest green build")
3. Scroll down and choose the download which matches your architecture:
   ![Actions](action_download.png "Choose download")
4. Install just like the stable release files.

**Note:** In order to download the snapshot artifacts, you need a github account you must be logged in.

## Building SNClient From Source

Building snclient from source is covered in detail here: [Building from source](build)
