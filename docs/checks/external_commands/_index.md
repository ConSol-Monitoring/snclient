---
title: External scripts
weight: 2000
---

## External Script Integration

SNClient runs custom scripts and external programs to monitor Windows systems with plugins and manage them with event handlers.
This feature is equivalent to NSClient++'s `CheckExternalScripts` module.

### Enabling External Script Integration

Enable external scripts in the configuration file:

```ini
[/modules]
CheckExternalScripts = enabled
```

### Adding Custom Scripts

Add custom scripts with either the concise or verbose format:

**Concise Format**:

```ini
[/settings/external scripts/scripts]
my_check1 = check_custom.bat
my_check2 = myscripts\check_custom.bat
```

**Verbose Format**:

```ini
[/settings/external scripts/scripts/my_check1]
command = check_custom.bat

[/settings/external scripts/scripts/my_check2]
command = myscripts\check_custom.bat
```

Both formats add two commands, `my_check1` and `my_check2`, that run the `check_custom.bat` script.
Use the concise format unless you need options for an individual command. For individual options, use the verbose format.

In the concise format, the key is the command name.
In the verbose format, the last part of the section path is the command name.
The `command` key specifies what to execute.
Set options such as `timeout` and `allow arguments` in that section to override the global external script settings for the command.

### Multiline Commands

To split a long command across lines, end each continued line with a space or tab followed by `\`.
For example:

```ini
[/settings/external scripts/scripts]
check_myscript = ${scripts}/myscript.sh \
          --arg1 foo \
          --arg2 bar
```

SNClient joins the lines with a single space before it parses the command.
See [multiline values](../../configuration/#multiline-values) for the syntax rules.

### Handling Script Arguments

Put script arguments directly in the command, or enable argument pass-through.
To enable argument pass-through, use these settings:

```ini
[/settings/external scripts]
allow arguments = true
```

Arguments are available in macros:

- `$ARGS$` contains all arguments, separated by spaces, **without** quotes.
- `$ARGS"$` contains all arguments, separated by spaces, **with** quotes.
- `$ARGSn$` contains the value of the argument at position `n`.
- `$ARGSn"$` contains the value of the argument at position `n` **with** quotes.

For security reasons, avoid argument pass-through. If you need arguments, use quoted arguments such as `$ARGS"$`.
Unquoted arguments almost always lead to a command injection vulnerability.

### Configuration Reference

#### External Script Integration Settings

- **allow arguments**: Allow or disallow script arguments when executing external scripts. Default is `false`.
- **allow nasty characters**: Permit or restrict certain characters (```|`&><'"\[]{}...```) in arguments. Default is `false`.
- **allow control characters**: Permit or restrict ASCII control characters in arguments. Default is `false`.
- **timeout**: Set the maximum execution time for commands (in seconds). This applies to external commands only, not internal ones.

```ini
[/settings/external scripts]
allow arguments = false
allow nasty characters = false
allow control characters = false
timeout = 60
```

#### Command Aliases

Create aliases for existing commands with arguments to simplify their use. Do not create loops in alias definitions.

```ini
[/settings/external scripts/alias/sample-alias]
alias = sample-alias
command = original-command
```

#### External Scripts

Define the external scripts that SNClient can run. Use the format `command = script arguments`.

```ini
[/settings/external scripts/scripts/sample-script]
command = custom_script.bat
```

Scripts with the extensions .bat, .ps1, or .exe (Windows), or .sh or no extension (Unix), can be defined as follows.

```ini
check_dummy = check_dummy.bat
check_dummy_ok = check_dummy.ps1 0 "i am ok"
check_dummy_critical = check_dummy.exe 2 "i am critical"
check_dummy_arg = check_dummy.exe "$ARG1$" "$ARG2$"
# for scripts with variable arguments
check_dummy_args = check_dummy.bat $ARGS$
check_dummy_args% = C:/Program Files/snclient/scripts/check_dummy.exe %ARGS%
# put variable arguments in quotes
check_dummy_argsq = ${scripts}/check_dummy.ps1 $ARGS"$
restart_service = NET START "$ARG1$"
```

For scripts in the `${scripts}` folder, use relative paths as shown in the examples.
SNClient resolves these paths to absolute paths before it runs the scripts.
Before it runs a script, SNClient sets the working directory to ${shared-dir}.


#### Script Discovery

```
; Load all scripts in a given folder - Load all (${script path}/*) scripts in a given directory and use them as commands.
; Any executable file is considered a script. They do not need to have a script extension.
; On Windows, executable files cannot be identified this way, so SNClient adds every file in the directory.
; Links are followed, and if they end up under the script path, they will be added as a script.
; If the path ends with **, its subdirectories will be searched as well.
script path = ${scripts}/autoinclude
```

Use `script path` to let SNClient discover and add scripts automatically.
Each alias uses the script's name and extension. You cannot change these aliases.

#### Wrapped Scripts

Define templates for script commands in the wrappings section.
SNClient expands these templates for scripts in the Wrapped Scripts section.
Use `%SCRIPT%` for the script and `%ARGS%` for its arguments.

```ini
[/settings/external scripts/wrappings]
vbs = cscript.exe /nologo %SCRIPT% %ARGS%
bat = cmd /c %SCRIPT% %ARGS%
ps1 = powershell.exe -ExecutionPolicy Bypass -File %SCRIPT% %ARGS%

[/settings/external scripts/wrapped scripts]
check_dummy_wrapped_noparm = check_dummy.ps1
check_dummy_wrapped = check_dummy.bat $ARG1$ "$ARG2$"
check_dummy_wrapped_ok = check_dummy.bat 0 "i am ok wrapped"
check_dummy_wrapped_critical = check_dummy.vbs 2 "i am critical wrapped"
```

**Note:** Unlike NSClient++, you don't need to use wrapping for PowerShell scripts.
