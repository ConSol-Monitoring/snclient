---
title: ManagedExporter
linkTitle: Managed
weight: 350
tags:
  - prometheus
  - exporter
---

Managed exporters are exporters started and managed by SNClient. They will get
a unique assigned url in the main webserver of SNClient.

Enable the managed exporters in the modules section:

```ini
[/modules]
ManagedExporterServer = enabled
```

You can then create exporters like this:

```ini
[/settings/ManagedExporter/example]
; agent path - sets path to prometheus-exporter binary
agent path = c:\Program Files\prometheus/example_exporter.exe

; agent args - sets additional arguments for the node exporter
agent args = --web.listen=127.0.0.1:9990

; agent port - sets internal listen address (ex.: --web.listen-address=)
agent address = 127.0.0.1:9990

; agent max memory - set a memory limit for the agent (agent will be restarted if the rss is higher, set to 0 to disabled)
;agent max memory = 256M

; agent user - set user this agent should run as (requires root permissions)
;agent user = nobody

; port - Port to use for the node exporter.
port = ${/settings/WEB/server/port}

; use ssl - This option controls if SSL will be enabled.
use ssl = ${/settings/WEB/server/use ssl}

; url prefix - set prefix to provided urls (/exportername)
url prefix = /example

; url match - set pattern which will forwarded to the exporter (use * to forward all urls below the prefix)
url match = /metrics
```

SNClient will then start the exporter automatically and will watch its memory usage.
The metrics can be scaped from `https://<ip>:8443/example/metrics`.

By default only the `/metrics` url will be forwarded to the exporter.
In case the exporter uses a custom path, you have to adjust the url match
attribute or set it to `*` to simply passthrough all urls (under the `url prefix`).
