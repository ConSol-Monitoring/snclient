---
title: check_os_version
---

# check_os_version

Checks the version of the host OS.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Metrics](#metrics)

### Implementation

| Windows | Linux | FreeBSD | MacOSX |
| --- | --- | --- | --- |
| :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |

## Examples

### **Default check**

    check_os_version
    OK - Microsoft Windows 10 Pro 10.0.19045.2728 Build 19045.2728 (arch: amd64)

## Argument Defaults

| Argument | Default Value |
| --- | --- |
top-syntax | \${status} - \${platform} \${version} (arch: \${arch}) |

## Metrics

#### **Check specific metrics**

| Metric | Description |
| --- | --- |
| platform | Platform of the OS |
| family | OS Family |
| version | Full version number |
| arch | OS architecture |