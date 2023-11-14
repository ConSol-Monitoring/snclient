---
title: Linux
linkTitle: Linux
weight: 340
tags:
  - prometheus
  - linux
  - node_exporter
  - exporter
---

## Node Exporter

When running SNClient+ on linux there is a builtin node exporter which can be
enabled by:

    [/modules]
    NodeExporterServer = enabled

    [/settings/NodeExporter/server]
    ; use same port as the web server
    port = ${/settings/WEB/server/port}

    ; disable password protection
    password =

You can then scrape linux metrics from `http://<ip>:8443/node/metrics`.

The node exporter will run as user `nobody` unless you set `agent user` otherwise.

The prometheus scrape config might look like this:

    - job_name: 'linux'
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

SNClient will monitor the node exporter memory usage and restart the exporter if
it exceeds the `agent max memory`. The default is 256MB.
