---
title: tasksched
---

## check_tasksched

Check status of scheduled jobs

- [Examples](#examples)
- [Argument Defaults](#argument-defaults)
- [Attributes](#attributes)

## Implementation

| Windows            | Linux | FreeBSD | MacOSX |
|:------------------:|:-----:|:-------:|:------:|
| :white_check_mark: |       |         |        |

## Examples

### Default Check

    check_tasksched
    OK - All tasks are ok

### Example using NRPE and Naemon

Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  check_tasksched
        use                  generic-service
        check_command        check_nrpe!check_tasksched!'crit=exit_code != 0'
    }

## Argument Defaults

| Argument      | Default Value                            |
| ------------- | ---------------------------------------- |
| filter        | enabled = true                           |
| warning       | exit_code != 0                           |
| critical      | exit_code < 0                            |
| empty-state   | 1 (WARNING)                              |
| empty-syntax  | %(status) - No tasks found               |
| top-syntax    | %(status) - \${problem_list}             |
| ok-syntax     | %(status) - All tasks are ok             |
| detail-syntax | \${folder}/\${title}: \${exit_code} != 0 |

## Check Specific Arguments

| Argument | Description                                                |
| -------- | ---------------------------------------------------------- |
| timezone | Sets the timezone for time metrics (default is local time) |

## Attributes

### Filter Keywords

these can be used in filters and thresholds (along with the default attributes):

| Attribute            | Description                                                        |
| -------------------- | ------------------------------------------------------------------ |
| application          | Name of the application that the task is associated with           |
| comment              | Comment or description for the work item                           |
| creator              | Creator of the work item                                           |
| enabled              | Flag whether this job is enabled (true/false)                      |
| exit_code            | The last jobs exit code                                            |
| exit_string          | The last jobs exit code as string                                  |
| folder               | Task folder                                                        |
| has_run              | True if this task has ever been executed                           |
| max_run_time         | Maximum length of time the task can run                            |
| most_recent_run_time | Most recent time the work item began running                       |
| priority             | Task priority                                                      |
| title                | Task title                                                         |
| hidden               | Indicates that the task will not be visible in the UI (true/false) |
| missed_runs          | Number of times the registered task has missed a scheduled run     |
| task_status          | Task status as string                                              |
| next_run_time        | Time when the registered task is next scheduled to run             |
| parameters           | Command line parameters for the task                               |
