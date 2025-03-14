---
title: ExporterExporter
linkTitle: ExpExp
weight: 360
tags:
  - prometheus
  - exporter
---

The exporter exporter is a simple reverse proxy for prometheus exporters not
managed by SNClient.

It makes multiple exporters accessible via a single proxy url.

Example configuration:

```ini
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
```

The exporter uses the standard http settings with an additional `modules dir` to
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

## Incompatibilities

### No Verification

This module basically works the same as the standalone exporter exporter, except
it does not implement verification. Files and requests are passed through as is
and not checked if it contains valid prometheus metrics.

### Changed Index Path

Since this exporter exporter (optionally) shares the web server with the rest of
the SNClient, the `/` url path is in use already. The available exporter
modules can therefore be requested with the `/list` path.
