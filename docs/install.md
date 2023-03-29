# Installation

## Building SNClient From Source

### Requirements

- go >= 1.19
- make

## Building

	%> git clone https://github.com/sni/snclient
	%> cd snclient
	%> make

## Building RPMs

Building RPM packages is available with the rpm make target:

	%> make rpm

You will need those extra requirements:

- rpm-build
- rpmlint
- help2man

## Building DEBs

Building Debian packages is available with the deb make target:

	%> make deb

You will need those extra requirements:

- dpkg
- lintian
- help2man
