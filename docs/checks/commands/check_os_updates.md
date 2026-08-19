---
title: os_updates
---

## check_os_updates

Checks for OS system updates.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux              | FreeBSD | MacOSX             |
|:------------------:|:------------------:|:-------:|:------------------:|
| :white_check_mark: | :white_check_mark: |         | :white_check_mark: |

## Examples

### Default Check

    check_os_updates
    OK - no updates available |...

If you only want to be notified about security related updates:

    check_os_updates warn=none crit='count_security > 0'
    CRITICAL - 1 security updates / 3 updates available. |'security'=1;;0;0 'updates'=3;0;;0

On DNF/APT systems, **--update** refreshes repository metadata in a private cache
owned by the SNClient service user unless started as root user.

The DNF check returns **UNKNOWN** if an enabled repository is unavailable, because
otherwise an incomplete repository set could be reported as having no updates.

Use **--max-metadata-age** to make the check return **UNKNOWN** if the repository
metadata has not been refreshed within the given duration, ex.: --max-metadata-age=24h

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_os_updates
        use                  generic-service
        check_command        check_nrpe!check_os_updates!warn='count > 0' crit='count_security > 0'
    }

## Argument Defaults

| Argument      | Default Value                                                                         |
| ------------- | ------------------------------------------------------------------------------------- |
| warning       | count > 0                                                                             |
| critical      | count_security > 0                                                                    |
| empty-state   | 0 (OK)                                                                                |
| empty-syntax  | %(status) - no updates available                                                      |
| top-syntax    | %(status) - %{count_security} security updates / %{count} updates available.\n%{list} |
| ok-syntax     |                                                                                       |
| detail-syntax | \${prefix}\${package}: \${version}                                                    |

## Check Specific Arguments

| Argument               | Description                                                                                  |
| ---------------------- | -------------------------------------------------------------------------------------------- |
| -m\|--max-metadata-age | Fail with UNKNOWN if the repository metadata (apt/yum/dnf) is older than this duration, ex.: 24h (default: disabled) |
| -s\|--system           | Package system: auto, apt, yum, osx and windows (default: auto)                              |
| -u\|--update           | Update package list (if supported, ex.: apt-get update)                                      |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute | Description                      |
| --------- | -------------------------------- |
| package   | package name                     |
| security  | is this a security update: 0 / 1 |
| version   | version string of package        |
