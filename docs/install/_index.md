---
linkTitle: Installation
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

Usually you should stick to the stable releases unless told otherwise or you want
to test something.

Installing the development snapshot is straight forward:

1. Open https://github.com/ConSol-Monitoring/snclient/actions/workflows/cicd.yml
2. Choose first green build like in
	![Actions](actions.png "Choose latest green build")
3. Scroll down and choose the download which matches your architecture:
	![Actions](action_download.png "Choose download")
4. Install just like the stable release files.


## Building SNClient From Source

### Requirements

- go >= 1.20
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
- help2man (for ex. from homebrew)
