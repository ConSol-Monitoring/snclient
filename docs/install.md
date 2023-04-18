# Installation

## Building SNClient From Source

### Requirements

- go >= 1.19
- make

## Building Binary

	%> git clone https://github.com/sni/snclient
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
