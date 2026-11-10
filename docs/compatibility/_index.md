---
title: Compatibility
linkTitle: Compatibility
---

SNClient aims to remain compatible with NSClient++, but some behavior differs.
SNClient keeps useful features but no longer supports some others.

## Configuration

![Change](../icons/changed.png "this is different in SNClient")

SNClient reads INI files differently from NSClient++.
It reads configuration files from top to bottom. If a setting appears more than once, the last value wins.

Keep default values in the main configuration. Use an include at the end to customize local settings.

### Modules

![Feature](../icons/feature.png "this is a new thing in SNClient")

You can enable modules in the global `[/modules]` section.
Most modules with their own configuration section can also use that section to enable the module.
This keeps related settings in one place.

For example:

```ini
[/modules]
NodeExporterServer = enabled
```

is the same as

```ini
[/settings/NodeExporter/server]
NodeExporterServer = enabled
```

If a module's own section contains this setting, it overrides the value from the modules section.

### Includes

SNClient supports configuration includes from local folders or HTTPS URLs.

Use includes to organize your configuration. Keep the default configuration unchanged and override values in a separate file.

SNClient evaluates files from `[/includes]` directly in the order they appear.

SNClient supports wildcards and folders in includes as well, for example:

```bash
[/includes]
custom = local_*.ini
confd  = conf.d/
```

Read more about includes on the [configuration page](../configuration/#includes).

## Performance Data

### Threshold Ranges

![Feature](../icons/feature.png "this is a new thing in SNClient")

SNClient supports performance data that follows [the official plugin developer guidelines](https://www.monitoring-plugins.org/doc/guidelines.html#AEN201) from Monitoring Plugins.

In addition to standard thresholds, SNClient supports ranges such as `10:20` or `@10:20`.

See examples here:
[threshold format](https://www.monitoring-plugins.org/doc/guidelines.html#THRESHOLDFORMAT)

Use the latest [check_snclient plugin](https://github.com/ConSol-Monitoring/check_snclient) (formerly check_nsc_web) to run checks.
Previous releases did not fully support those ranges.

### Bytes

![Change](../icons/changed.png "this is different in SNClient")

All byte-related performance data is now reported in bytes.

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

SNClient simplifies `perf-config`. It processes configuration options in order and applies only the first match.

See the [performance configuration page](../checks/perfconfig/) for details.

SNClient's perf-config has an additional `magic` option to apply a magic factor.

## Allow Arguments Handling

![Feature](../icons/feature.png "this is a new thing in SNClient")

SNClient accepts arguments from alias definitions even when `allow arguments` is disabled.
In this case, it blocks only additional arguments.

For example: this works, even with `allow arguments` disabled.

```ini
[/settings/external scripts/alias]
alias_test = check_cpu warn=load=80 crit=load=90
```

If you enable arguments, use quoted arguments like `$ARGS"$`.
Unquoted arguments lead to command injection vulnerabilities.

## Allow Nasty Characters Handling

![Change](../icons/changed.png "this is different in SNClient")

In addition to the existing characters, SNClient does not allow the `$`, `;` and newline characters.

The list of disallowed characters is therefore:

```txt
$|`&><'"\[]{};\n\r
```

Change the list of nasty chars with the `nasty characters` configuration option.

```ini
[/settings/default]
nasty characters = $|`&><'"\\[]{};\n\r
```

Do not remove default characters. Each character protects against attacks.
Removing characters might lead to command injection vulnerabilities.

## Allow Control Characters Handling

![Feature](../icons/feature.png "this is a new thing in SNClient")

In addition to the existing nasty characters, SNClient does not allow ASCII control characters.

Change the default value with the `allow control characters` configuration option if you need to.

```ini
[/settings/default]
allow control characters = false
```

## Allowed Hosts Handling

![Feature](../icons/feature.png "this is a new thing in SNClient")

All network services, including NRPE and the REST web server, support `allowed hosts`.

## TLS/SSL Configuration

![Change](../icons/changed.png "this is different in SNClient")

SNClient simplifies TLS configuration. Set a minimum required TLS version instead of specific ciphers.

For example:

```ini
[/settings/default]
tls min version = "tls1.3"
```

The default, TLS 1.2 or higher, disables known insecure ciphers.
If you allow TLS below 1.2, SNClient enables all ciphers, including insecure ones.

See the cipher list on [go.dev](https://pkg.go.dev/crypto/tls#pkg-constants).

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

The following sections describe changes to individual checks.

### check_drivesize

![Change](../icons/changed.png "this is different in SNClient")

When some drives are unavailable, `check_drivesize` handles multiple drives differently from NSClient++.
SNClient requires all specified paths to exist.

Example:

```bash
check_drivesize drive=c drive=f

UNKNOWN - failed to find disk partition: CreateFile f:: The system cannot find the path specified.
```

If only one of the drives needs to exist, use a filter like this:

```bash
check_drivesize filter="letter='C' or letter='F'"

OK - All 1 drive(s) are ok |....
```

### check_files

![Change](../icons/changed.png "this is different in SNClient")

`check_files` has one metric for each of `access, creation and written` time, instead of separate metrics for local and utc timezones.
Use the timezone argument to set the timezone. The default is the local timezone.

Example:

```bash
check_files path=c:/windows warn=size>2MB max-depth=1 timezone=Europe/Berlin
```

### check_process

![Change](../icons/changed.png "this is different in SNClient")

The `check_process` behaves differently when checking for processes that are not running.

```bash
check_process process=noneexisting.exe crit="state = 'started'" empty-state=0
OK - no processes found with this filter. |...
```

Add `empty-state=0` to return OK when no process exists.
If a process exists, SNClient uses the normal critical expression.

### check_service

![Feature](../icons/feature.png "this is a new thing in SNClient")

If the service is running, `check_service` adds memory and CPU metrics.

```bash
check_service service=snclient
OK - All 1 service(s) are ok. |'snclient'=4;;;; 'snclient rss'=12943360B;;;; 'snclient vms'=6492160B;;;; 'snclient cpu'=0%;;;;
```

### check_tasksched

![Feature](../icons/feature.png "this is a new thing in SNClient")

Use the timezone argument to set the timezone for `check_tasksched`.
The default is local time.
