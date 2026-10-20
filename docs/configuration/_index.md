---
title: Configuration Files
linkTitle: Configuration
---

## General

SNClient uses the INI-style configuration format with some enhancements ([see syntax specification below](#syntax)).

The default configuration is explained in the [packaging file](https://github.com/ConSol-Monitoring/snclient/blob/main/packaging/snclient.ini).

## File Locations

By default SNClient searches for a `snclient.ini` in the following folders:

- `./snclient.ini` (current folder)
- `/etc/snclient/snclient.ini`
- `${exe-path}/snclient.ini` (this is `C:\Program Files\snclient\snclient.ini` on Windows)

## Custom Configuration

The default config file includes a wildcard pattern `snclient_local*.ini` which
makes it easy to include local custom configuration. Putting custom configuration
in a separate file has a couple of advantages.

- no conflicts during package updates.
- clean separation of upstream and user configuration.
- no need to walk through the whole default configuration to see what has been customized.

Best practice is to create a `snclient_local.ini` file, for example:

```ini
[/modules]
CheckExternalScripts = enabled

[/settings/default]
allowed hosts = 127.0.0.1, 10.0.1.2
password = SHA256:9f86d081884...
```

See the [includes section](#includes) for details about including files.

### Windows

The location for a custom file would be: `C:\Program Files\snclient\snclient_local.ini`

### Linux

The location for a custom file would be: `/etc/snclient/snclient_local.ini`

## Syntax

The configuration uses the INI file format. For example:

```ini
[/settings/default]
allowed hosts = 127.0.0.1, ::1
```

The maximum length of a single line in the INI file is limited to 1MB.

### Comments

Lines starting with `#` or `;` are treated as comments and ignored. You can use
both characters in values.

```ini
[/settings/external scripts]
# this is a comment
check_echo = echo '# this is not a comment and printed as is.'
```

### Quotes

Quotes are optional, even for text.

```ini
[/settings/default]
tls min version = tls1.2
password        = "CHANGEME"
```

There is no difference between single and double quotes. Macros will
be interpolated in both variants.

Quotes will be removed from the value if the value starts and ends with them.

```ini
[/settings/external scripts]
# will be used without quotes
check_remove_quotes = "${script}/test.sh"

# quotes will not be trimmed here since they don't surround the value:
check_keep_quotes = "C:\Program Files\snclient\snclient.exe" -V

# quotes will also be kept here:
check_also_keep = "part 1" something else "part 2"

# this will throw an error
check_wrong = "unclosed quotes
```

### Multiline Values

End a line with a space or tab followed by `\` to continue its value on the next
line. The backslash and surrounding whitespace, including indentation on the next
line, are replaced with a single space. This works for configuration values in
all sections, including external commands and `check_multi` commands.

For example, check the size of a backup folder:

```ini
[/settings/check/multi/backups]
command[size] = check_files \
          "path=/srv/backups" \
          "warn=total_size > 10GiB"
```

Run `snclient run check_multi config=backups`.

A continuation must be followed immediately by a nonempty line. A blank or
whitespace-only line, or the end of the file, causes an `unfinished line
continuation` error. Windows paths such as `C:\scripts\` remain unchanged because
the final backslash is not preceded by a space or tab.

See [external commands](../checks/external_commands/#multiline-commands) for an
external command example.

### Appending Values

You may use the `+=` operator to append to existing values and write
more readable configuration files.

```ini
[/settings/default]
allowed hosts  = 127.0.0.1, ::1
allowed hosts += , 192.168.0.1
allowed hosts += , 192.168.0.2,192.168.0.3
```

Values will simply be joined as text, so in case you want to create lists, make sure you
add a comma. (Starting with v0.38 this will be done automatically)

## Inheritance

The configuration is split into multiple sections, but in order to
avoid having duplicate password or allowed hosts entries, inheritance can
be used to define shared settings only once.

```ini
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
```

Each section inherits values from its default section,
from parent sections and their default sections.

This is the order of inheritance for the example above:

- /settings/sub1/other (most significant)
- /settings/sub1/default
- /settings/sub1
- /settings/default (least significant)

The first defined value will be used.

## Includes

It is possible and encouraged to include other INI files to organize your settings.

Includes can either be local files or HTTP(S) URLs to load remote configuration files.

### Local Includes

For local includes it is recommended to make use of the default include path `snclient_local*.ini`.

```ini
[/includes]
local = snclient_local*.ini
```

### HTTPS Includes

HTTPS URLs can also be used to load centrally managed configuration
files. It is recommended to use HTTPS links and use some sort of authentication.

The simple way:

```ini
[/includes]
remote = https://user:password@central.company/snclient/default.ini
```

If you want to configure more details, create a new section for each include and
adjust all default HTTP client options:

```ini
[/includes/company]
url                  = https://central.company/snclient/default.ini
#user                = test        # set a username to authenticate
#password            = changeme    # use a password
#insecure            = false       # skip hostname verification
#request timeout     = 60          # change http request timeout
#client certificate  = client1.crt # use client certificates to authenticate
#certificate key     = client1.key # private key for client certificate
```

The HTTP include will be locally cached, so SNClient will still start, even if the
remote server is temporarily not available. The agent will try to update the include
on reload and daemon startup. The SNClient will not try to update the include
for simple commands, such as `snclient run ...` or `snclient hash`, unless the include
has never been downloaded.

In case the include cannot be downloaded and no local cache is present, the agent
will refuse to start.

## Macros

Macros can be used in the INI file configuration to access path variables.

Supported macro variants:

- `${macro name}`
- `%(macro name)`
- all variants of `$` / `%` and `{}` / `()` are interchangeable

### Available Macros

Basically all the values from the `[/paths]` section are available as macros
plus some more.

List of available macros:

| Suffix                | Example                                    | Description                                     |
| --------------------- | ------------------------------------------ | ------------------------------------------------|
| `${certificate-path}` | usually `${shared-path}`                   | Directory of certificates (set by snclient.ini) |
| `${exe-file}`         | snclient or snclient.exe                   | Basename of executable                          |
| `${exe-full}`         | /usr/bin/snclient                          | Full path to executable                         |
| `${exe-path}`         | /usr/bin or c:\Program Files\snclient      | Directory of executable                         |
| `${file-ext}`         | .exe or empty                              | File extension of agent's executable file        |
| `${goarch}`           | x86_64, aarch64, i386, ...                 | The value of Go's runtime.GOARCH             |
| `${goos}`             | linux, windows, darwin, ...                | The value of Go's runtime.GOOS               |
| `${hostname}`         | machine.local                              | Name of the host running the agent              |
| `${pkgarch}`          | amd64, arm64, 386, ...                     | Architecture as used in package file names      |
| `${scripts}`          | usually `${shared-path}/scripts`           | Directory of script files (set by snclient.ini) |
| `${shared-path}`      | /etc/snclient or c:\Program Files\snclient | Directory of shared files (set by snclient.ini) |

## On-Demand Macros

Besides the path macros, you can access and reference any configuration value
with on-demand macros:

- `${/settings/section/attribute}`
- `%(/settings/section/attribute)`

For example, add a dummy check which returns the allowed hosts setting for the
webserver component:

```ini
[/settings/external scripts/alias]
alias_allowed_hosts = check_dummy 0 "weballowed:${/settings/WEB/server/allowed hosts}"
```

On-demand macros are only available during the initial config parsing and will
not be used for plugin arguments for security reasons.

## Macro Operators

The list of available macro operators is defined [here](../checks/macros/).

For example, to define a dummy command which prints the hostname in lowercase letters:

```ini
[/settings/external scripts/alias]
alias_hostname = check_dummy 0 "host:${hostname:lc}"
```
