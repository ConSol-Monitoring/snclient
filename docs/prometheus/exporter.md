---
title: Exporter Exporter
linkTitle: Exporter Exporter
description: Proxy requests to Prometheus exporters that are not managed by SNClient.
weight: 360
tags:
  - prometheus
  - exporter
---

The exporter exporter is a simple reverse proxy for Prometheus exporters not
managed by SNClient.

It exposes multiple exporters through a single proxy URL.

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

The exporter uses the standard HTTP settings. Set `modules dir` to specify the folder that contains the exported module definitions.

For compatibility, modules use the YAML format described on [GitHub](https://github.com/QubitProducts/exporter_exporter#configuration).

Example HTTP exporter module file:

exporter_modules/name.yaml:

```yml
method: http
http:
    port: 9100
    path: '/metrics'
```

If you use the default ports, test the exporter with this command:

```sh
%> curl 'https://hostname:8443/proxy?module=name'
```

## Endpoints

The exporter provides the following endpoints:

- `/proxy`: used to query the selected exporter. Accepts these parameters:
  - `module`: use `?module=name` parameter to specify the exporter.
  - `args`: arguments for exec exporters.
  - SNClient passes other arguments through to HTTP exporters.
- `/list`: list available exporters.

## Incompatibilities

### No Verification

This module works like the standalone exporter exporter, except that it does not verify metrics.
It passes files and requests through unchanged, without checking for valid Prometheus metrics.

### Changed Index Path

The embedded exporter exporter can share SNClient's web server, where the `/` URL path is already in use.
Use `/list` to request the available exporter modules.

### No separate /metrics endpoint

The embedded exporter exporter has no separate /metrics endpoint.
