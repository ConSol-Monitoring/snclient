---
title: Built-in plugins
weight: 2000
---

Built-in check plugins work like the official [monitoring-plugins](https://www.monitoring-plugins.org/).

Most plugins have a help page. To open it, use `-h` or `--help`.

For example:

```bash
./snclient run check_snclient --help
```

Unlike normal checks, check plugins do not support filtering.

See the built-in checks [in the commands section](../commands/).

## Enabling Builtin Plugins

Enable built-in plugins in the configuration file:

```ini
[/modules]
CheckBuiltinPlugins = enabled
```

## Available Plugins
