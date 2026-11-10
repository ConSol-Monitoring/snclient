---
title: Prometheus
linkTitle: Prometheus
weight: 300
tags:
  - prometheus
  - exporter
---

SNClient supports several Prometheus integrations.

## Overview

- [Built-in Windows Exporter](#windows-exporter)
- [Built-in Node Exporter](#node-exporter)
- [Managed Exporters](#managed-exporters)
- [ExporterExporter](#exporter-exporter)
- [Prometheus Metrics of SNClient](#metrics)

## Windows Exporter

SNClient includes a built-in Windows exporter on Windows.

[Read more about the windows_exporter](windows).

## Node Exporter

SNClient includes a built-in node exporter on Linux and macOS.

[Read more about the node_exporter](node).

## Managed Exporters

SNClient starts and manages these exporters. Each exporter gets a unique URL on SNClient's main web server.

[Read more about managed exporters](managed).

## Exporter Exporter

The exporter_exporter (expexp) is a reverse proxy for existing exporters that SNClient does not manage.
[Read more about this exporter](exporter).

## Metrics

SNClient also exports Prometheus metrics about its own process.

Enable these metrics in the modules section of `snclient.ini`:

```ini
[/modules]
PrometheusServer = enabled

[/settings/Prometheus/server]
port = 9999
use ssl = false
password =
```

You can then scrape Prometheus metrics from `http://<ip>:9999/metrics`.
