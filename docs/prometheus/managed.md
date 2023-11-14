---
title: ManagedExporter
linkTitle: Managed
weight: 350
tags:
  - prometheus
  - exporter
---

Managed exporters are exporters started and managed by SNClient+. They will get
a unique assigned url in the main webserver of SNClient+.

Enable the managed exporters in the modules section:

    [/modules]
    ManagedExporterServer = enabled

You can then create exporters like this:

    ;[/settings/ManagedExporter/example]
    agent path = c:\Program Files\prometheus/example_exporter.exe
    agent args = --web.listen=127.0.0.1:9990
    agent address = 127.0.0.1:9990
    port = ${/settings/WEB/server/port}
    use ssl = ${/settings/WEB/server/use ssl}
    url prefix = /example

SNClient+ will then start the exporter automatically and watch its memory usage.
The metrics can be scaped from `https://<ip>:8443/example/metrics`.
