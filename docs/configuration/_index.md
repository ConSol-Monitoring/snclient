---
title: Configuration Files
linkTitle: Configuration
---

## General

SNClient uses the INI configuration format with extensions. See the [syntax specification](#syntax).

The default configuration is explained in the [packaging file](https://github.com/ConSol-Monitoring/snclient/blob/main/packaging/snclient.ini).

## File Locations

By default, SNClient searches for `snclient.ini` in these locations:

- `./snclient.ini` (current folder)
- `/etc/snclient/snclient.ini`
- `${exe-path}/snclient.ini` (this is `C:\Program Files\snclient\snclient.ini` on Windows)

## Custom Configuration

The default configuration includes local files that match `snclient_local*.ini`.
Keep custom settings in a separate file to:

- Avoid conflicts during package updates.
- Separate upstream settings from user settings.
- Review custom settings without searching the default configuration.

Create a `snclient_local.ini` file, for example:

```ini
[/modules]
CheckExternalScripts = enabled

[/settings/default]
allowed hosts = 127.0.0.1, 10.0.1.2
password = SHA256:9f86d081884...
```

See the [includes section](#includes) for details about including files.

### Windows

Put the custom file at: `C:\Program Files\snclient\snclient_local.ini`

### Linux

Put the custom file at: `/etc/snclient/snclient_local.ini`

## Syntax

The configuration uses the INI file format. For example:

```ini
[/settings/default]
allowed hosts = 127.0.0.1, ::1
```

The maximum line length in the INI file is 1MB.

### Comments

SNClient ignores lines that start with `#` or `;` as comments. You can use both characters in values.

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

Single and double quotes work the same way. SNClient expands macros in both.

If a value starts and ends with quotes, SNClient removes the surrounding quotes.

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

To continue a value on the next line, end the line with a space or tab followed by `\`.
SNClient replaces the backslash and surrounding whitespace with a single space. This includes indentation on the next line.
Continuation works in all sections, including external commands and `check_multi` commands.

For example, check the size of a backup folder:

```ini
[/settings/check/multi/backups]
command[size] = check_files \
          "path=/srv/backups" \
          "warn=total_size > 10GiB"
```

After a continuation, the next line must be nonempty.
A blank or whitespace-only line causes an `unfinished line
continuation` error. The end of the file causes the same error.
Windows paths such as `C:\scripts\` remain unchanged because no space or tab precedes the final backslash.

See [external commands](../checks/external_commands/#multiline-commands) for an
external command example.

### Appending Values

Use the `+=` operator to append text to existing values.

```ini
[/settings/default]
allowed hosts  = 127.0.0.1, ::1
allowed hosts += , 192.168.0.1
allowed hosts += , 192.168.0.2,192.168.0.3
```

SNClient joins values as text. To create a list, add a comma between values.
Starting with v0.38, SNClient adds the comma automatically.

## Inheritance

The configuration has multiple sections. Use inheritance to define shared settings, such as passwords or allowed hosts, only once.

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

SNClient uses the first defined value.

## Includes

Include other INI files to organize your settings.

Use local files or HTTP(S) URLs for includes.

### Local Includes

For local includes, use the default include path `snclient_local*.ini`.

```ini
[/includes]
local = snclient_local*.ini
```

### HTTPS Includes

Use HTTPS URLs to load centrally managed configuration files. We recommend HTTPS with authentication.

The simple way:

```ini
[/includes]
remote = https://user:password@central.company/snclient/default.ini
```

To customize HTTP client options, create a section for each include:

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

SNClient caches HTTP includes locally so that it can start when the remote server is temporarily unavailable.
The agent tries to update includes on reload and daemon startup.
For simple commands, such as `snclient run ...` or `snclient hash`, it downloads an include only if no previous download exists.

If SNClient cannot download an include and no local cache exists, it refuses to start.

## Macros

Use macros in INI files to access path variables.

Supported macro variants:

- `${macro name}`
- `%(macro name)`
- all variants of `$` / `%` and `{}` / `()` are interchangeable

### Available Macros

All values from the `[/paths]` section are available as macros, along with additional values.

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

Use on-demand macros to access any configuration value:

- `${/settings/section/attribute}`
- `%(/settings/section/attribute)`

For example, add a dummy check which returns the allowed hosts setting for the
webserver component:

```ini
[/settings/external scripts/alias]
alias_allowed_hosts = check_dummy 0 "weballowed:${/settings/WEB/server/allowed hosts}"
```

SNClient expands on-demand macros only when it first parses the configuration.
For security reasons, it does not expand them in plugin arguments.

## Macro Operators

See the [available macro operators](../checks/macros/).

For example, to define a dummy command which prints the hostname in lowercase letters:

```ini
[/settings/external scripts/alias]
alias_hostname = check_dummy 0 "host:${hostname:lc}"
```

## Environment Variables

Configuration values can reference environment variables with the `$ENV{NAME}` syntax:

```ini
[/settings/default]
allowed hosts = 127.0.0.1, $ENV{SNCLIENT_ALLOWED_HOSTS}

[/settings/WEB/server]
password = $ENV{SNCLIENT_WEB_PASSWORD}

[/paths]
shared-path = $ENV{SNCLIENT_SHARED_PATH}

[/includes]
node = $ENV{SNCLIENT_NODE_INI}
```

SNClient reads the variable from the environment of the running process when it
loads the configuration. You can distribute the same configuration files and
only change the environment per node.

- An unset variable is a configuration error. `snclient config check` reports
  the variable name and config location.
- A variable that is set but empty expands to an empty string.
- SNClient expands each value only once. It does not expand `$ENV{...}` inside
  the value of an environment variable again.
- Use `$$ENV{NAME}` to write a literal `$ENV{NAME}`.
- SNClient expands only config values. It does not expand section names and keys.
- The syntax is only available in configuration files, not in check output
  templates or runtime macros.
- SNClient does not re-read environment variables on config reload. Restart the
  service after you change them. One-shot commands pick up the environment of
  the new process automatically.

On Linux with systemd, set variables with `Environment=` or `EnvironmentFile=` in
the service unit. On Windows, set them in the service or system environment and
restart the service.

The local agent parses HTTP(S) includes, so it resolves environment variables
in remote config files on the client.
