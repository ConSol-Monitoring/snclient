---
title: Prometheus
linkTitle: Prometheus
weight: 300
tags:
  - prometheus
  - exporter
---

There are several prometheus integrations in SNClient+.

## Overview

- [Builtin Windows Exporter](#windows-exporter)
- [Builtin Node Exporter](#node-exporter)
- [Managed Exporters](#managed-exporters)
- [ExporterExporter](#exporter-exporter)
- [Prometheus Metrics of SNClient+](#metrics)

## Windows Exporter

When running SNClient+ on windows there is a builtin windows exporter.

[Read more about the windows exporter](windows).

## Node Exporter

When running SNClient+ on linux there is a builtin node exporter.

[Read more about the windows exporter](linux).

## Managed Exporters

Managed exporters are exporters started and managed by SNClient+. They will get
a unique assigned url in the main webserver of SNClient+.

[Read more about managed exporters](managed).

## Exporter Exporter

The exporter_exporter (expexp) is a reverse proxy for already existing exporters.
[Read more about this exporter](exporter).

The exporter exporter is for exporters not managed by SNClient+.

## Metrics

SNClient+ itself is a prometheus exporter as well and provides some metrics
about the agent process.

It can be enabled in the modules section of the `snclient.ini`.

    [/modules]
    PrometheusServer = enabled

    [/settings/Prometheus/server]
    port = 9999
    use ssl = false
    password =

You can then scrape prometheus metrics from `http://<ip>:9999/metrics`.
