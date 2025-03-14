---
title: Compatibility
linkTitle: Compatibility
---

SNClient tries hard to be backwards compatible with the NSClient++, however there are a couple of things different. Some things which seemed useful and do no harm, others do not make sense anymore.

So here is a list of things different to NSClient++.

## Configuration

![Change](../icons/changed.png "this is different in SNClient")

The way SNClient reads the ini files is different and more intuitive. Config files
will be read from top to bottom and if the same value is encountered again it
will be overwritten. The last value wins.

That way you can have a default config with all default values and use a simple
include at the end to customize local settings.

Included files from `[/includes]` are evaluated directly in the order of appearance.

SNClient supports wildcards and folders in includes as well, for example:

```bash
[/includes]
custom = local_*.ini
confd  = conf.d/
```

## Performance Data

### Threshold Ranges

![Feature](../icons/feature.png "this is a new thing in SNClient")

SNClient supports standard monitoring-plugins compatible performance data as
described in [the official plugin developer guidelines](https://www.monitoring-plugins.org/doc/guidelines.html#AEN201).

In addition to standard thresholds, SNClient added support for ranges, ex.: `10:20` or `@10:20`.

See examples here:
[threshold format](https://www.monitoring-plugins.org/doc/guidelines.html#THRESHOLDFORMAT)

It is advised to always use the latest [check_nsc_web plugin](https://github.com/ConSol-Monitoring/check_nsc_web) to do the checks.
Previous releases did not fully support those ranges.

### Bytes

![Change](../icons/changed.png "this is different in SNClient")

All bytes related performance data uses bytes in the performance data now.

### SI Units

![Change](../icons/changed.png "this is different in SNClient")

SNClient uses IEC units if possible. This means for example:

| Unit | Description                       |
| ---- | --------------------------------- |
| GB   | 1000000000 bytes (base 10) 1000^3 |
| GiB  | 1073741824 bytes (base 2)  1024^3 |
| Gb   | same as GiB                       |
| G    | same as GB                        |

### Perf-Config

![Change](../icons/changed.png "this is different in SNClient")

The `perf-config` option has been simplified. Multiple config options are applied
in order and only the first match is applied.

A detailed description can be found on the [performance configuration page](../checks/perfconfig/).

SNClients perf-config has an additional `magic` option to apply a magic factor.

## Allow Arguments Handling

![Feature](../icons/feature.png "this is a new thing in SNClient")

SNClient allows arguments from alias definitions, even if `allow arguments` is not allowed. Only additional arguments
are not allowed in this case.

Ex.: this works, even with `allow arguments` disabled.

```ini
[/settings/external scripts/alias]
alias_test = check_cpu warn=load=80 crit=load=90
```

## Allow Nasty Characters Handling

![Change](../icons/changed.png "this is different in SNClient")

In addition to the existing characters, SNClient does not allow the `$` character.

The list of not allowed nasty characters is therefore:

```txt
$|`&><'\"\\[]{}
```

Change the list of nasty chars with the `nasty characters` configuration option.

```ini
[/settings/default]
nasty characters = $|`&><'\"\\[]{}
```

## Allowed Hosts Handling

![Feature](../icons/feature.png "this is a new thing in SNClient")

The `allowed hosts` configuration is available for all network services, not only NRPE. The REST webserver can make use
of it as well.

## TLS/SSL Configuration

![Change](../icons/changed.png "this is different in SNClient")

The tls configuration has been simplified. Instead of setting specific ciphers, you can now set a
minimum required tls version.

ex.:

```ini
[/settings/default]
tls min version = "tls1.3"
```

Allowing tls 1.2 or higher (default) disables known insecure ciphers. Allowing
tls lower than 1.2 enables all ciphers (even the insecure ones).

The list of ciphers can be found on [go.dev](https://pkg.go.dev/crypto/tls#pkg-constants).

## Passwords

![Feature](../icons/feature.png "this is a new thing in SNClient")

SNClient supports using hashed passwords in the configuration file.

```bash
%> snclient hash
enter password to hash or hit ctrl+c to exit.
<entering password>
hash sum: SHA256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08
```

Then use this hash as password:

```ini
[/settings/WEB/server]
password = SHA256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08
```

## Checks

Check specific changes and enhancements:

### check_drivesize

![Change](../icons/changed.png "this is different in SNClient")

The `check_drivesize` handles multiple drives different if some are not available. SNClient requires all specified paths to exist.

Example:

```bash
check_drivesize drive=c drive=f

UNKNOWN - failed to find disk partition: CreateFile f:: The system cannot find the path specified.
```

If the drives are optional as long as one drive exists, you could make use of a filter like this:

```bash
check_drivesize filter="letter='C' or letter='F'"

OK - All 1 drive(s) are ok |....
```

### check_files

![Change](../icons/changed.png "this is different in SNClient")

`check_files` now only has one metric for `access, creation and written` time instead of multiple for local and utc timezones. As a replacement it has the option to set the timezone by using the timezone argument. The default is the local timezone.

Example:

```bash
check_files path=c:/windows warn=size>2MB max-depth=1 timezone=Europe/Berlin
```

### check_process

![Change](../icons/changed.png "this is different in SNClient")

The `check_process` behaves a bit different when checking for not running processes.

```bash
check_process process=noneexisting.exe crit="state = 'started'" empty-state=0
OK - no processes found with this filter. |...
```

The trick is to add `empty-state=0` so in case there is no process, the result will be OK. And in
case there is a process, the normal critical expression will be used.

### check_service

![Feature](../icons/feature.png "this is a new thing in SNClient")

The `check_service` adds memory and cpu metrics if the service is running.

```bash
check_service service=snclient
OK - All 1 service(s) are ok. |'snclient'=4;;;; 'snclient rss'=12943360B;;;; 'snclient vms'=6492160B;;;; 'snclient cpu'=0%;;;;
```

### check_tasksched

![Feature](../icons/feature.png "this is a new thing in SNClient")

`check_tasksched` also has the option to set the timezone using the timezone argument.
The default is still local time.
