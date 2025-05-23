---
title: External Check Commands
weight: 2000
---

## External Script Integration

**Overview:**

The SNClient agent provides a flexible and extensible solution for executing custom scripts and external programs to monitor (plugins) and manage (event handlers) your Windows systems. This guide will walk you through configuring and utilizing this feature, which is equivalent to NSClient++'s `CheckExternalScripts` module.

### Enabling External Script Integration

To enable the external script integration feature in SNClient, you need to activate it in the config file as follows:

```ini
[/modules]
CheckExternalScripts = enabled
```

### Adding Custom Scripts

You can add your custom scripts to SNClient using either a concise or verbose format. Here are examples of both:

**Concise Format**:

```ini
[/settings/external scripts]
my_check1 = check_custom.bat
my_check2 = myscripts\check_custom.bat
```

**Verbose Format**:

```ini
[/settings/external scripts/scripts/my_check1]
my_check1 = check_custom.bat

[/settings/external scripts/scripts/my_check2]
my_check2 = myscripts\check_custom.bat
```

Both formats achieve the same outcome by adding two new commands, `my_check1` and `my_check2`, which execute the `check_custom.bat` script. Usually you use the short format, but if you want to provide individual options to a command, the long format is the way to go.

### Handling Script Arguments

You can manage script arguments in two ways: embedding them directly into the command or allowing for argument pass-through. To enable argument pass-through, update the configuration as follows:

```ini
[/settings/external scripts]
allow arguments = true
```

Arguments are available in macros:

- `$ARGS$` contains all macros space separated **without** quotes.
- `$ARGS"$` contains all macros space separated **with quotes**.
- `$ARGSn$` contains the value of the argument at position `n`.

### Configuration Reference

Below, you'll find a reference section for configuring the External Script Integration feature of SNClient

#### External Script Integration Settings

- **allow arguments**: Allow or disallow script arguments when executing external scripts. Default is `false`.
- **allow nasty characters**: Permit or restrict certain potentially dangerous characters (```|`&><'"\[]{}```) in arguments. Default is `false`.
- **timeout**: Set the maximum execution time for commands (in seconds). This applies to external commands only, not internal ones.

```ini
[/settings/external scripts]
allow arguments = false
allow nasty characters = false
timeout = 60
```

#### Command Aliases

You can create aliases for existing commands with arguments to simplify usage. Ensure that you don't create loops in alias definitions.

```ini
[/settings/external scripts/alias/sample-alias]
alias = sample-alias
command = original-command
```

#### External Scripts

Define scripts available for execution via the External Script Integration feature. Use the format `command = script arguments`.

```ini
[/settings/external scripts/scripts/sample-script]
command = custom_script.bat
```

Scripts with an extension of .bat, .ps1 and .exe (Windows) or .sh and no extension at all (Unix) can be defined as follows.

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

If your scripts are located within the `${scripts}` folder, you can specify them using relative paths, as demonstrated in the examples. SNClient will automatically obtain the absolute path for these scripts and use it for execution. Prior to running the scripts, SNClient configures the working directory to be ${shared-dir}.

#### Wrapped Scripts

Specify script templates used to define script commands. These templates are expanded by scripts located in the Wrapped Scripts section. Use `%SCRIPT%` to represent the actual script and `%ARGS%` for any provided arguments.

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

**Note:** Unlike NSClient++, you don't need to use wrapping for Powershell scripts.
