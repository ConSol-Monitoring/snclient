---
title: check_tasksched
---

# check_tasksched

Check status of scheduled jobs.

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Metrics](#metrics)

### Implementation

| Windows | Linux | FreeBSD | MacOSX |
|:-------:|:-----:|:-------:|:------:|
| :construction: | :x: | :x: | :x: |

## Examples

### **Default check**

    check_tasksched warn='exit_code != 0'
    OK: All tasks are ok

## Argument Defaults

| Argument | Default Value |
| --- | --- |
empty-state | 1 (Warning) |
top-syntax | \${status}: ${problem_list} |
ok-syntax | %(status): All tasks are ok |
empty-syntax | %(status): No tasks found |
detail-syntax | \${folder}/\${title}: ${exit_code} != 0 |

### **Check specific arguments**

| Argument | Description |
| --- | --- |
| timezone | Sets the timezone for time metrics (default is local time) |

## Metrics

#### **Check specific metrics**

| Metric | Description |
| --- | --- |
| application | Name of the application the task is associated with |
| comment | Description of the task |
| creator | Creator of the task |
| enabled | If the task is enabled |
| exit_code | exit code of the last run |
| exit_string | exit string of the last run |
| folder | Folder where the task is located |
| max_run_time | maximum length the task can run |
| most_recent_run_time | Time of the last run |
| priority | Priority of the task |
| title | Title of the task |
| hidden | If the task is hidden |
| missed_runs | Amount of runs the task missed |
| task_status | Status of the task|
| next_run_time | Time of the next scheduled run |