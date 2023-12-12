---
title: API
linkTitle: API
---

SNClient+ comes with a REST API which can be used to run checks or do administrative tasks.

For the sake of completeness the list of endpoints also contains the available none-rest URLs.

## Endpoints

### /

Redirects to `/index.hmtl`

### /index.html

Returns simple OK message:

Example:

    curl \
        -u user:changeme \
        -X POST \
        https://127.0.0.1:8443/index.html

Returns:

    snclient working...

### /metrics

Returns prometheus metics for snclient itself.

### /node/metrics

Used by the [WindowsExporter](../prometheus/windows/) and [NodeExporter](../prometheus/node/).

Returns prometheus metrics from those exporters.

### /list

Used by the [ExporterExporer](../prometheus/exporter/) server.

Returns list of exporters.

### /proxy

Used by the [ExporterExporer](../prometheus/exporter/) server.

Returns metrics for given exporter.

### /query/{command}

Legacy check command endpoint.

Example:

    curl \
        -u user:changeme \
        -X POST \
        https://127.0.0.1:8443/query/check_uptime

### /api/v1/queries/{command}/commands/execute

Version 1 check command endpoint.

Example:

    curl \
        -u user:changeme \
        -X POST \
        https://127.0.0.1:8443/api/v1/queries/check_uptime/commands/execute

### /api/v1/inventory

Returns the check inventory as json

Example:

    curl \
        -u user:changeme \
        -X POST \
        https://127.0.0.1:8443/api/v1/inventory

Returns:

    {
        "inventory": {
            "cpu": [
                {
                    "core": "core1",
                    "core_id": "core1",
                    "load": "43",
                    "time": "5m"
                },
                ...
            ],
            ...
            "temperature": [
                {
                    "crit": "100.000000",
                    "label": "Core 1",
                    "max": "100.000000",
                    "min": "0.000000",
                    "name": "coretemp",
                    "path": "/sys/class/hwmon/hwmon5/temp3_input",
                    "temperature": "44.000000"
                }
            ],
            "uptime": [
                {
                    "boot": "2023-12-04 07:16:01",
                    "uptime": "8d 09:07h",
                    "uptime_value": "724072.8"
                }
            ]
        },
        "localtime": 1702398235
    }

### /api/v1/admin/reload

Reload the configuration.

Example:

    curl \
        -u user:changeme \
        -X POST \
        https://127.0.0.1:8443/api/v1/admin/reload

Returns

    {"success":true}

### /api/v1/admin/certs/replace

Replace the TLS certificate and key on the fly. Reload is optional.

The certificates need to be base64 encoded as in the following example.

Example:

    curl \
        -u user:changeme \
        -d '{ "certdata": "'$(base64 -w 0 ./server.crt)'", "keydata": "'$(base64 -w 0 dist/server.key)'", "reload": true }' \
        https://127.0.0.1:8443/api/v1/admin/certs/replace

Returns

    {"success":true}
