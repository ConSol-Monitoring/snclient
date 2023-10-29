---
title: Configuration Files
linkTitle: Configuration
---

## General

SNClient+ uses the ini style config format.

## File Locations

By default SNClient searches for a `snclient.ini` in the following folders:

- `./snclient.ini` (current folder)
- `/etc/snclient/snclient.ini`
- `${exe-path}/snclient.ini` (this is `C:\Program Files\snclient\snclient.ini` on windows)

## Custom Configuration

The default config file includes a wildcard pattern `snclient_local*.ini` which
makes it easy to include local custom configuration. Putting custom configuration
in a seperate file has a couple of advantages.

- no conflicts during package updates.
- clean separation of upstream and user configuration.
- no need to walk through the hole default configuration to see what has been customized.

Best practice is to create a file `snclient_local.ini`, ex.: like this:

    [/modules]
    CheckExternalScripts = enabled

    [/settings/default]
    allowed hosts = 127.0.0.1, 10.0.1.2
    password = SHA256:9f86d081884...

### Windows

The location for a custom file would be: `C:\Program Files\snclient\snclient_local.ini`

### Linux

The location for a custom file would be: `/etc/snclient/snclient_local.ini`

## Inheritence

The configuration is splitted into multiple sections, but in order to
avoid having duplicate password or allowed hosts entries, inheritance can
be used to only specify central things once.

    [/settings/sub1/other]
    key = value

    [/settings/sub1/default]
    ; fallback if the above is not set
    key = value

    [/settings/sub1]
    ; fallback if the above is not set
    key = value

    [/settings/default]
    ; fallback if the above is not set
    key = value

Each section inherits values from it's default section,
from parent sections and parents defaults section.

This is the order of inheritance for the example above:

- /settings/sub1/other (most significant)
- /settings/sub1/default
- /settings/sub1
- /settings/default (least significant)

The first defined value will be used.

## Macros

Macros can be used in the ini file configuration to access path variables.

Supported macro variants:

- `${macroname}`
- `%(macroname)`

Available macros are:

- `${exe-path}`
- `${shared-path}`
- `${scripts}`
- `${certificate-path}`
- `${hostname}`

Basically the values from the `[/paths]` section and the hostname.

## On Demand Macros

Besides the path macros, you can access and reference any configuration value
with on demand macros:

- `${/settings/section/attribute}`
- `%(/settings/section/attribute)`

For example, add a dummy check which returns the allowed hosts setting for the
webserver component:

    [/settings/external scripts/alias]
    alias_allowed_hosts = check_dummy 0 "weballowed:${/settings/WEB/server/allowed hosts}"

On demand macros are only available during the initial config parsing and will
not be used for plugin arguments for security reasons.

## Macro Operators

Macro values can be altered by adding a colon separated suffix.

Support operators are:

- `:lc` - make value lowercase
- `:uc` - make value uppercase

for example, define a dummy command which prints the hostname in lower case letters:

    [/settings/external scripts/alias]
    alias_hostname = check_dummy 0 "host:${hostname:lc}"
