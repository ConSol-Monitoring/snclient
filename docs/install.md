---
title: Installation
---

# Installation

## Using Binary Packages

Using packages is the recommended way to install SNClient+.

### Stable Releases
Installation packages from stable releases can be found here:

- https://github.com/ConSol-Monitoring/snclient/releases

### Development Snapshots
During development each code commit produces build artifacts if all tests were
successful.

You will find those artifacts here:

- https://github.com/ConSol-Monitoring/snclient/actions/workflows/cicd.yml

Usually you should stick to the stable releases unless you want to test something.

## Building SNClient From Source

### Requirements

- go >= 1.19
- make

## Building Binary

	%> git clone https://github.com/Consol-Monitoring/snclient
	%> cd snclient
	%> make snclient

## Building RPMs

Building RPM packages is available with the `make rpm` target:

	%> make rpm

You will need those extra requirements:

- rpm-build
- help2man
- (rpmlint)

## Building DEBs

Building Debian packages is available with the `make deb` target:

	%> make deb

You will need those extra requirements:

- dpkg
- help2man
- (lintian)

## Building OSX PKG

Building OSX packages is available with the `make osx` target:

	%> make osx

You will need those extra requirements:

- Xcode
