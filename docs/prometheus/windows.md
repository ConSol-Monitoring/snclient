---
title: Windows Exporter
linkTitle: Windows Exporter
description: Enable SNClient's built-in Windows exporter to expose Windows metrics.
weight: 330
tags:
  - prometheus
  - windows
  - exporter
  - windows_exporter
---

## Windows Exporter

On Windows, enable the built-in Windows exporter with these settings:

```ini
[/modules]
WindowsExporterServer = enabled

[/settings/WindowsExporter/server]
; use same port as the web server
port = ${/settings/WEB/server/port}

; disable password protection
require password = false

; extend enabled default collectors
;agent args = --collectors.enabled=[defaults],time,license
```

You can then scrape Windows metrics from `http://<ip>:8443/node/metrics`.

Example Prometheus scrape configuration:

```yaml
- job_name: 'windows'
  # Override the global default and scrape targets from this job every 5 seconds.
  scrape_interval: 5s

  # metrics_path defaults to '/metrics', but here we use the snclient-prefix
  metrics_path: /node/metrics

  # scheme defaults to 'http'.
  scheme: https
  tls_config:
    insecure_skip_verify: true

  static_configs:
  - targets: ['<ip>:8443']
```

SNClient monitors the Windows exporter's memory usage.
If usage exceeds `agent max memory`, SNClient restarts the exporter. The default limit is 256MB.
