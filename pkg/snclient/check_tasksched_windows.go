//go:build windows

package snclient

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"syscall"
	"time"
)

//go:embed embed/scripts/windows/scheduled_tasks.ps1
var scheduledTasksPS1 string

func (l *CheckTasksched) addTasks(ctx context.Context, snc *Agent, check *CheckData) error {
	cmd := powerShellCmd(ctx, scheduledTasksPS1)
	output, stderr, exitCode, _, err := snc.runExternalCommand(ctx, cmd, DefaultCmdTimeout)
	if err != nil {
		return fmt.Errorf("getting scheduled tasks failed: %s\n%s", err.Error(), stderr)
	}
	if exitCode != 0 {
		return fmt.Errorf("getting scheduled tasks failed: %s\n%s", output, stderr)
	}

	var taskList []ScheduledTask
	err = json.Unmarshal([]byte(output), &taskList)
	if err != nil {
		return fmt.Errorf("could not unmarshal scheduled tasks: %s", err.Error())
	}

	for index := range taskList {
		task := taskList[index]
		hasRun := false
		if task.LastRunTime != "" {
			hasRun = true
		}
		parameters := ""

		entry := map[string]string{
			"application":          task.Name,
			"comment":              task.Description,
			"creator":              task.Author,
			"enabled":              fmt.Sprintf("%t", task.Enabled),
			"exit_code":            fmt.Sprintf("%d", task.LastTaskResult.exitCode()),
			"exit_string":          task.LastTaskResult.String(),
			"folder":               task.Path,
			"has_run":              fmt.Sprintf("%t", hasRun),
			"max_run_time":         task.TimeLimit,
			"most_recent_run_time": fmt.Sprintf("%d", l.parseDate(task.LastRunTime).Unix()),
			"priority":             fmt.Sprintf("%d", task.Priority),
			"title":                task.Name,
			"hidden":               fmt.Sprintf("%t", task.Hidden),
			"missed_runs":          fmt.Sprintf("%d", task.MissedRuns),
			"task_status":          task.State.String(),
			"next_run_time":        fmt.Sprintf("%d", l.parseDate(task.NextRunTime).Unix()),
			"parameters":           parameters,
		}
		check.listData = append(check.listData, entry)
	}

	return nil
}

func (l *CheckTasksched) parseDate(raw string) time.Time {
	// date matches the pattern /Date(unixmilliseconds))/
	if len(raw) > 6 && raw[0:6] == "/Date(" {
		unixMilliseconds, err := strconv.ParseInt(raw[6:len(raw)-2], 10, 64)
		if err == nil {
			return time.Unix(0, unixMilliseconds*int64(time.Millisecond))
		}
	}

	return time.Time{}
}

type TaskResult uint32

// https://learn.microsoft.com/en-us/windows/win32/taskschd/task-scheduler-error-and-success-constants
const (
	TaskResSuccess TaskResult = 0x0
	TaskResReady   TaskResult = iota + 0x00041300
	TaskResRunning
	TaskResDisabled
	TaskResHasNotRun
	TaskResNoMoreRuns
	TaskResNotScheduled
	TaskResTerminated
	TaskResNoValidTriggers
	TaskResEventTrigger
	TaskResSomeTriggersFailed TaskResult = 0x0004131B
	TaskResBatchLogonProblem  TaskResult = 0x0004131C
	TaskResQueued             TaskResult = 0x00041325
)

func (r TaskResult) String() string {
	switch r {
	case TaskResSuccess:
		return "Completed successfully"
	case TaskResReady:
		return "Ready"
	case TaskResRunning:
		return "Currently running"
	case TaskResDisabled:
		return "Disabled"
	case TaskResHasNotRun:
		return "Has not been run yet"
	case TaskResNoMoreRuns:
		return "No more runs scheduled"
	case TaskResNotScheduled:
		return "One or more of the properties that are needed to run this task on a schedule have not been set"
	case TaskResTerminated:
		return "Terminated by user"
	case TaskResNoValidTriggers:
		return "Either the task has no triggers or the existing triggers are disabled or not set"
	case TaskResEventTrigger:
		return "Event triggers do not have set run times"
	case TaskResSomeTriggersFailed:
		return "Not all specified triggers will start the task"
	case TaskResBatchLogonProblem:
		return "May fail to start unless batch logon privilege is enabled for the task principal"
	case TaskResQueued:
		return "Queued"
	default:
		return syscall.Errno(r).Error()
	}
}

//nolint:mnd // enumerated exit codes
func (r TaskResult) exitCode() int32 {
	switch r {
	case TaskResSuccess:
		return 0
	case TaskResReady:
		return 1
	case TaskResRunning:
		return 2
	case TaskResDisabled:
		return 3
	case TaskResHasNotRun:
		return 4
	case TaskResNoMoreRuns:
		return 5
	case TaskResNotScheduled:
		return 6
	case TaskResTerminated:
		return 7
	case TaskResNoValidTriggers:
		return 8
	case TaskResEventTrigger:
		return 9
	case TaskResSomeTriggersFailed:
		return 10
	case TaskResBatchLogonProblem:
		return 11
	case TaskResQueued:
		return 12
	default:
		return -1
	}
}

// TaskState specifies the state of a running or registered task.
// https://docs.microsoft.com/en-us/windows/desktop/api/taskschd/ne-taskschd-task_state
type TaskState uint

const (
	TaskStateUnknown  TaskState = iota // the state of the task is unknown
	TaskStateDisabled                  // the task is registered but is disabled and no instances of the task are queued or running. The task cannot be run until it is enabled
	TaskStateQueued                    // instances of the task are queued
	TaskStateReady                     // the task is ready to be executed, but no instances are queued or running
	TaskStateRunning                   // one or more instances of the task is running
)

func (t TaskState) String() string {
	switch t {
	case TaskStateUnknown:
		return "Unknown"
	case TaskStateDisabled:
		return "Disabled"
	case TaskStateQueued:
		return "Queued"
	case TaskStateReady:
		return "Ready"
	case TaskStateRunning:
		return "Running"
	default:
		return ""
	}
}

type ScheduledTask struct {
	Name           string                `json:"TaskName"`
	Path           string                `json:"TaskPath"`
	Description    string                `json:"Description"`
	LastRunTime    string                `json:"LastRunTime"`
	State          TaskState             `json:"State"`
	NextRunTime    string                `json:"NextRunTime"`
	LastTaskResult TaskResult            `json:"LastTaskResult"`
	MissedRuns     int64                 `json:"MissedRuns"`
	Author         string                `json:"Author"`
	Enabled        bool                  `json:"Enabled"`
	Priority       int64                 `json:"Priority"`
	Hidden         bool                  `json:"Hidden"`
	TimeLimit      string                `json:"TimeLimit"`
	Actions        []ScheduledTaskAction `json:"Actions"`
}

type ScheduledTaskAction struct {
	Execute   string `json:"Execute"`
	Arguments string `json:"Arguments"`
}
