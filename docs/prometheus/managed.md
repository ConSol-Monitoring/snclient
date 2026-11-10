---
title: Managed Exporters
linkTitle: Managed Exporters
description: Configure SNClient to start, monitor, and expose managed Prometheus exporters.
weight: 350
tags:
  - prometheus
  - exporter
---

SNClient starts and manages these exporters. Each exporter gets a unique URL on SNClient's main web server.

Enable the managed exporters in the modules section:

```ini
[/modules]
ManagedExporterServer = enabled
```

Then configure each exporter. For example:

```ini
[/settings/ManagedExporter/example]
; agent path - sets path to prometheus-exporter binary
agent path = c:\Program Files\prometheus/example_exporter.exe

; agent args - sets additional arguments for the node exporter
agent args = --web.listen=127.0.0.1:9990

; agent address - sets internal listen address (ex.: --web.listen-address=)
agent address = 127.0.0.1:9990

; agent max memory - set a memory limit for the agent (agent will be restarted if the rss is higher, set to 0 to disable)
;agent max memory = 256M

; agent user - set user this agent should run as (requires root permissions)
;agent user = nobody

; port - Port to use for the node exporter.
port = ${/settings/WEB/server/port}

; use ssl - This option controls if SSL will be enabled.
use ssl = ${/settings/WEB/server/use ssl}

; url prefix - set prefix to provided urls (/exportername)
url prefix = /example

; url match - set pattern that will be forwarded to the exporter (use * to forward all urls below the prefix)
url match = /metrics
```

SNClient starts the exporter automatically and monitors its memory usage.
Scrape the metrics from `https://<ip>:8443/example/metrics`.

By default, SNClient forwards only the `/metrics` URL to the exporter.
If the exporter uses a custom path, adjust `url match`.
Alternatively, set it to `*` to forward all URLs under `url prefix`.
