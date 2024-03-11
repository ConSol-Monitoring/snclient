---
title: Configuration Files
linkTitle: Configuration
---

## General

SNClient+ uses the ini style config format ([see syntax specification below](#syntax)).

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

## Syntax

The configuration uses the ini file format. For example:

    [/settings/default]
    allowed hosts = 127.0.0.1, ::1

The maximum length of a single line in the ini file is limited to 1MB.

### Comments

Lines starting with `#` or `;` are comments and ignored. You can use
both characters as value.

    [/settings/external scripts]
    # this is a comment
    check_echo = echo '# this is not a comment and printed as is.'

### Quotes

Quotes are optional, even for text.

    [/settings/default]
    tls min version = tls1.2
    password        = "CHANGEME"

There is no difference between single and double quotes. Macros will
be interpolated in both variants.

Quotes will be removed from the value if the value starts and ends with them.

    [/settings/external scripts]
    # will be used without quotes
    check_remove_quotes = "${script}/test.sh"

    # quotes will not be trimmed here since they don't surround the value:
    check_keep_quotes = "C:\Program Files\snclient\snclient.exe" -V

    # quotes will also be kept here:
    check_also_keep = "part 1" something else "part 2"

    # this will throw an error
    check_wrong = "unclosed quotes

### Appending Values

You may use the `+=` operator to append to existing values and write
more readable configuration files.

    [/settings/default]
    allowed hosts  = 127.0.0.1, ::1
    allowed hosts += , 192.168.0.1
    allowed hosts += , 192.168.0.2,192.168.0.3

Values will simply be joined as text, so in case you want to create lists, make sure you
add a comma.

## Inheritance

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
- all variants of `$` / `%` and `{}` / `()` are interchangeable

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

| Suffix                | Example                               | Description |
| --------------------- | ------------------------------------- | ------------|
| `:lc`                 | Word -> word                          | make value lowercase |
| `:uc`                 | test -> TEST                          | make value uppercase |
| `:h`                  | 1000 -> 1k                            | make a number more human readably |
| `:duration`           | 125 -> 2m 5s                          | convert amount of seconds into human readable duration |
| `:age`                | 1700834034 -> 374                     | substract value from current unix timestamp to get the age in seconds |
| `:date`               | 1700834034 -> 2023-11-24 14:53:54 CET | convert unix timestamp into human readable date (local timezone) |
| `:utc`                | 1700834034 -> 2023-11-24 13:53:54 UTC | convert unix timestamp into human readable date (utc timezone) |
| `:fmt=<fmt>`          | 123.45 -> 123.4                       | apply format, ex.: $(total \| fmt=%.1f) (using GOs fmt.Sprintf) |
| `:s/regexp/replace/`  | C:\ % -> C                            | apply regexp replacement, ex.: $(name \| 's/\\W//' ) (using GOs regexp.Compile) |
| `:ascii`              | C:\ % -> C                            | remove any none-ascii characters |

for example, define a dummy command which prints the hostname in lower case letters:

    [/settings/external scripts/alias]
    alias_hostname = check_dummy 0 "host:${hostname:lc}"

Operators can be put together:

    $(datemacro:date:uc)

This converts $(datemacro) to a human readable date and make everything uppercase.

You can also use the pipe symbol to use multiple operators in a row, ex.:

    $(macroname | date | uc)

Use the generic fmt operator to apply any format on numbers, ex.:

    $(used_pct | fmt=%d)
