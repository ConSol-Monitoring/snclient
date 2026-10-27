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

When running SNClient on Windows there is a built-in Windows exporter which can be
enabled by:

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

The Prometheus scrape config might look like this:

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

SNClient will monitor the Windows exporter's memory usage and restart the exporter if
it exceeds the `agent max memory` limit. The default is 256MB.
