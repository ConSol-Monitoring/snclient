---
title: Getting Started
linkTitle: Getting Started
weight: 300
---

This guide sets up basic Windows monitoring with SNClient for CPU usage, physical
memory usage, and drive C: disk usage.

The recommended way is to use [OMD Labs](https://omd.consol.de/docs/omd/), which already includes
[Naemon](https://omd.consol.de/docs/naemon/), the [Thruk](https://omd.consol.de/docs/thruk/) web interface, and the `check_snclient` plugin.
Naemon uses this plugin to query the SNClient REST API over HTTPS.

## Install SNClient on Windows

Follow the [installation instructions](../install/), specifically the
[Windows installation](../install/windows/), to install the MSI package.
During installation:

- Keep the REST web server enabled with HTTPS on port **8443**.
- Set a password and allow the IP address of your OMD monitoring server.
- Ensure the monitoring server can reach TCP port 8443 on the Windows host.
  The MSI installer creates the Windows firewall exception.

For example, run the following from an elevated command prompt in the directory
containing the downloaded MSI (replace its filename, IP address, and password):

```bat
msiexec.exe /i snclient.msi ALLOWEDHOSTS="127.0.0.1,::1,192.0.2.10" PASSWORD="your-password"
```

Here, `192.0.2.10` is the monitoring server. The built-in CPU and disk checks are
enabled by default; no external scripts are needed for these checks.

## Prepare OMD

Install OMD Labs on a Linux monitoring server using the
[OMD installation instructions](https://omd.consol.de/docs/omd/#installation).
If you do not already have a site, create one and start it.

Run the following Linux commands as the OMD site user. For an existing site,
switch to its user with `omd su <site>` instead.

The plugin is located at `~/lib/monitoring-plugins/check_snclient`. Test the
connection, replacing `192.0.2.20` with the Windows host's address and using the
password you set during installation:

```sh
~/lib/monitoring-plugins/check_snclient -k -p 'your-password' -u 'https://192.0.2.20:8443'
~/lib/monitoring-plugins/check_snclient -k -p 'your-password' -u 'https://192.0.2.20:8443' check_cpu
~/lib/monitoring-plugins/check_snclient -k -p 'your-password' -u 'https://192.0.2.20:8443' check_memory "type=physical"
~/lib/monitoring-plugins/check_snclient -k -p 'your-password' -u 'https://192.0.2.20:8443' check_drivesize "drive=C:\\"
```

The first command should report that the REST API is reachable. If it fails,
check the password, allowed hosts, and firewall settings before continuing.

These examples use `-k` for the certificate shipped with SNClient, which skips
certificate verification. For verified TLS, configure a trusted server
certificate and remove `-k`; use the plugin's `-ca` option for a private CA.
See the [check_snclient options](../checks/plugins/check_snclient/) for details.

## Configure Naemon

Create `~/etc/naemon/conf.d/windows.cfg` with the following configuration.
Replace the address and password with the values used above. The example uses
the OMD `generic-host` and `generic-service` templates. In OMD, `$USER1$` points
to `~/lib/monitoring-plugins`.

```text
define command {
    command_name         check_windows_snclient
    command_line         $USER1$/check_snclient -k -p '$_HOSTSNCLIENT_PASSWORD$' -u 'https://$HOSTADDRESS$:8443' $ARG1$ $ARG2$
}

define host {
    use                  generic-host
    host_name            windows-server
    alias                Windows Server
    address              192.0.2.20
    _SNCLIENT_PASSWORD   your-password
    check_command        check_windows_snclient
}

define service {
    use                  generic-service
    host_name            windows-server
    service_description  CPU
    check_command        check_windows_snclient!check_cpu!'warn=load > 80' 'crit=load > 90'
}

define service {
    use                  generic-service
    host_name            windows-server
    service_description  Memory
    check_command        check_windows_snclient!check_memory!type=physical 'warn=used > 80%' 'crit=used > 90%'
}

define service {
    use                  generic-service
    host_name            windows-server
    service_description  Drive C
    check_command        check_windows_snclient!check_drivesize!drive=C: 'warn=used > 80%' 'crit=used > 90%'
}
```

The host check tests REST API connectivity. The three services warn above 80% usage and become critical above 90%.

Validate the configuration and, if validation succeeds, reload Naemon:

```sh
naemon -v ~/etc/naemon/naemon.cfg
omd reload naemon
```

Open the site's Thruk interface at `https://<monitoring-server>/<site>/thruk/`.
After the first checks run, the `windows-server` host and its CPU, Memory, and
Drive C services should show their status and performance data.

For more options, see [check_cpu](../checks/commands/check_cpu/),
[check_memory](../checks/commands/check_memory/), and
[check_drivesize](../checks/commands/check_drivesize/).

A full list of available check commands of SNClient can be found [here](../checks/).
