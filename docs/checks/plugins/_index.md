---
title: Check Plugins
weight: 2000
---

This is the list of built-in check plugins. They work like the official [monitoring-plugins](https://www.monitoring-plugins.org/).

Each plugin usually has a help page which can be accessed by the `-h` or `--help` argument.

For example:

```bash
./snclient run check_snclient --help
```

Check plugins cannot use filtering as the normal checks.

The list of built-in checks can be found [in the commands section](../commands/).

## Enabling Builtin Plugins

To enable the builtin plugins feature in SNClient, you need to activate them in the config file as follows:

```ini
[/modules]
CheckBuiltinPlugins = enabled
```

## Available Plugins
