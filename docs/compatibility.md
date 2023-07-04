---
title: Compatibility
---

# Compatibility

SNClient+ tries hard to be backwards compatible with the NSClient++, however there are a couple of things different. Some things which seemed useful and do no harm, others do not make sense anymore.

So here is a list of things different to NSClient++.

## Performance Data
### Threshold Ranges
<img src="./icons/feature.png">

SNClient+ supports standard monitoring-plugins compatible performance data as
described in https://www.monitoring-plugins.org/doc/guidelines.html#AEN201.

In addition to standard thresholds, SNClient+ added support for ranges, ex.: `10:20` or `@10:20`.

See examples here:
https://www.monitoring-plugins.org/doc/guidelines.html#THRESHOLDFORMAT

It is advised to always use the latest [check_nsc_web plugin](https://github.com/ConSol-Monitoring/check_nsc_web) to do the checks. Previous releases did not fully support those ranges.


### Bytes
<img src="./icons/changed.png">

All bytes related performance data uses bytes in the performance data now.


### SI Units
<img src="./icons/changed.png">

SNClient uses IEC units if possible. This means for example:

	GB  = 1000000000 bytes (base 10) 1000^3
	GiB = 1073741824 bytes (base 2)  1024^3
	Gb  = same as GiB
	G   = same as GB


## Allow Arguments Handling
<img src="./icons/feature.png">

SNClient allows arguments from alias definitions, even if `allow arguments` is not allowed. Only additional arguments
are not allowed in this case.

Ex.: this works, even with `allow arguments` disabled.

	[/settings/external scripts/alias]
	alias_test = check_cpu warn=load=80 crit=load=90

## Allow Nasty Characters Handling
<img src="./icons/changed.png">

In addition to the existing characters, SNClient does not allow the `$` character.

The list of not allowed nasty characters is therefore:

	$|`&><'\"\\[]{}

Change the list of nasty chars with the `nasty characters` configuration option.

	[/settings/default]
	nasty characters = $|`&><'\"\\[]{}

## Allowed Hosts Handling
<img src="./icons/feature.png">

The `allowed hosts` configuration is available for all network services, not only NRPE. The REST webserver can make use
of it as well.

## TLS/SSL Configuration
<img src="./icons/changed.png">

The tls configuration has been simplified. Instead of setting specific ciphers, you can now set a
minimum required tls version.

ex.:

	[/settings/default]
	tls min version = "tls1.3"

Allowing tls 1.2 or higher (default) disables known insecure ciphers. Allowing
tls lower than 1.2 enables all ciphers.

## Passwords
<img src="./icons/feature.png">
SNClient supports using hashed passwords in the configuration file.

	%> snclient hash
	enter password to hash or hit ctrl+c to exit.
	<entering password>
	hash sum: SHA256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08

Then use this hash as password:

	[/settings/WEB/server]
	password = SHA256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08



## Checks

Check specific changes and enhancements:

### check_service
<img src="./icons/feature.png">

The `check_service` adds memory and cpu metrics if the service is running.

	check_service service=snclient
	OK: All 1 service(s) are ok. |'snclient'=4;;;; 'snclient rss'=12943360B;;;; 'snclient vms'=6492160B;;;; 'snclient cpu'=0%;;;;
