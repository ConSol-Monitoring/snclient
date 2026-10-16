---
title: ExporterExporter
linkTitle: ExpExp
weight: 360
tags:
  - prometheus
  - exporter
---

The exporter exporter is a simple reverse proxy for Prometheus exporters not
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

The exporter uses the standard HTTP settings with an additional `modules dir` to
configure the exported modules.

For compatibility reasons, the modules themselves are in YAML format as described
on [GitHub](https://github.com/QubitProducts/exporter_exporter#configuration).

A simple HTTP exporter module file could look like this:

exporter_modules/name.yaml:

```yml
method: http
http:
    port: 9100
    path: '/metrics'
```

Using the default ports, you can test the exporter above with the command:

```sh
%> curl 'https://hostname:8443/proxy?module=name'
```

## Endpoints

The exporter provides the following endpoints:

- `/proxy`: used to query the selected exporter. Accepts these parameters:
  - `module`: use `?module=name` parameter to specify the exporter.
  - `args`: arguments for exec exporters.
  - other arguments are passed through to HTTP exporters.
- `/list`: list available exporters.

## Incompatibilities

### No Verification

This module basically works the same as the standalone exporter exporter, except
it does not implement verification. Files and requests are passed through unchanged,
without checking whether they contain valid Prometheus metrics.

### Changed Index Path

Since this exporter exporter (optionally) shares the web server with the rest of
the SNClient, the `/` URL path is in use already. The available exporter
modules can therefore be requested with the `/list` path.

### No separate /metrics endpoint

The embedded exporter exporter has no separate /metrics endpoint.
