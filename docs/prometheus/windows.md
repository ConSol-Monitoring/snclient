---
title: Windows
linkTitle: Windows
weight: 330
tags:
  - prometheus
  - windows
  - exporter
  - windows_exporter
---

## Windows Exporter

When running SNClient+ on windows there is a builtin windows exporter which can be
enabled by:

    [/modules]
    WindowsExporterServer = enabled

    [/settings/WindowsExporter/server]
    ; use same port as the web server
    port = ${/settings/WEB/server/port}

    ; disable password protection
    password =

You can then scrape windows metrics from `http://<ip>:8443/node/metrics`.

The prometheus scrape config might look like this:

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

SNClient will monitor the windows exporter memory usage and restart the exporter if
it exceeds the `agent max memory`. The default is 256MB.
