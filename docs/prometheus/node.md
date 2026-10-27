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

When running SNClient on Linux or macOS there is a built-in node exporter which can be
enabled by:

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

The node exporter will run as user `nobody` unless you set `agent user` otherwise.

The Prometheus scrape config might look like this:

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

SNClient will monitor the node exporter's memory usage and restart the exporter if
it exceeds the `agent max memory` limit. The default is 256MB.
