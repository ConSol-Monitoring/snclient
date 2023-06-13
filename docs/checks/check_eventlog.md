---
title: check_eventlog
---

# check_eventlog

Check for errors in the eventlog.

- [Argument Defaults](#argument-defaults)
- [Metrics](#metrics)

### Implementation

| Windows | Linux | FreeBSD | MacOSX |
| --- | --- | --- | --- |
| :construction: | :x: | :x: | :x: |

## Argument Defaults

| Argument | Default Value |
| --- | --- |
empty-state | 3 (Unknown) |
top-syntax | %(count) message(s) %(problem_list) |
ok-syntax | Event log seems fine |
empty-syntax | No entries found |
detail-syntax | %(file) %(source) (%(message)) |

### **Check specific arguments**

| Argument | Description |
| --- | --- |
| file | File to read (can be specified multiple times to check multiple files) |
| log | Alias for file |

## Metrics

#### **Check specific metrics**

| Metric | Description |
| --- | --- |
| computer | Which computer generated the message |
| file | The logfile name |
| log | Alias for file |
| id | Eventlog id |
| keyword | Keyword associated with the event |
| level | Severity level |
| message | The message as a string |
| source | The source system |
| provider | Alias for source |
| task | The type of event |
| written | Time of the message being written |