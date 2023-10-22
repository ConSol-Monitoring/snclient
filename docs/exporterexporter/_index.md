---
title: ExporterExporter
linkTitle: ExporterExporter
weight: 300
tags:
  - prometheus
  - exporter
---

The exporter exporter is a simple reverse proxy for prometheus exporter.

Example configuration:

    [/modules]
    ExporterExporterServer = enabled

    [/settings/ExporterExporter/server]
    ; port - Port to use for exporter_exporter.
    port = 8443

    ; use ssl - This option controls if SSL will be enabled.
    use ssl = true

    ; url prefix - set prefix to provided urls
    url prefix = /

    ; modules dir - set folder with yaml module definitions
    modules dir = ${shared-path}/exporter_modules

The exporter uses tha standard http settings with an additional `modules dir` to
configure the exported modules.

For compatibility reasons, the modules itself are in yaml format as described
on [github](https://github.com/QubitProducts/exporter_exporter#configuration).

A simple http exporter module file could look like:

exporter_modules/http.yaml:

    node:
        method: http
        http:
            port: 9100
            path: '/metrics'
