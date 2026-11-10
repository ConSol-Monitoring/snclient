---
title: Node Exporter (Linux/macOS)
linkTitle: Node Exporter (Linux/macOS)
description: Enable SNClient's built-in node exporter to expose Linux and macOS metrics.
weight: 340
tags:
  - prometheus
  - linux
  - macos
  - node_exporter
  - exporter
---

## Node Exporter

On Linux or macOS, enable the built-in node exporter with these settings:

```ini
[/modules]
NodeExporterServer = enabled

[/settings/NodeExporter/server]
; use same port as the web server
port = ${/settings/WEB/server/port}

; disable password protection
require password = false
```

You can then scrape Linux metrics from `http://<ip>:8443/node/metrics`.

The node exporter runs as user `nobody` unless you set a different `agent user`.

Example Prometheus scrape configuration:

```yaml
- job_name: 'node'
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

SNClient monitors the node exporter's memory usage.
If usage exceeds `agent max memory`, SNClient restarts the exporter. The default limit is 256MB.
