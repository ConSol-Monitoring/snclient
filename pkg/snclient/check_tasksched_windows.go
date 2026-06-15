//go:build windows

package snclient

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"syscall"
	"time"

	"github.com/goccy/go-json"
)

//go:embed embed/scripts/windows/scheduled_tasks.ps1
var scheduledTasksPS1 string

func (l *CheckTasksched) addTasks(ctx context.Context, snc *Agent, check *CheckData) error {
	script := scheduledTasksPS1
	cmd := powerShellCmd(ctx, script)
	err := powerShellCmdAddVariableDefinition(cmd, "title", l.TaskTitle)
	if err != nil {
		return err
	}
	err = powerShellCmdAddVariableDefinition(cmd, "folder", l.Folder)
	if err != nil {
		return err
	}
	err = powerShellCmdAddVariableDefinition(cmd, "recursive", strconv.FormatBool(l.Recursive))
	if err != nil {
		return err
	}

	output, stderr, exitCode, _, err := snc.runExternalCommand(ctx, cmd, snc.getBuiltinCmdTimeout())
	if err != nil {
		return fmt.Errorf("getting scheduled tasks failed, error: %s\n%s", err.Error(), stderr)
	}
	if exitCode != 0 {
		return fmt.Errorf("getting scheduled tasks failed, exitCode: %d\n%s", exitCode, stderr)
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
			"creator":              task.UserID,
			"enabled":              fmt.Sprintf("%t", task.Enabled),
			"exit_code":            fmt.Sprintf("%d", task.LastTaskResult.exitCode()),
			"exit_string":          task.LastTaskResult.String(),
			"folder":               task.Path,
			"has_run":              fmt.Sprintf("%t", hasRun),
			"max_run_time":         task.ExecutionTimeLimit,
			"most_recent_run_time": fmt.Sprintf("%d", l.parseDate(task.LastRunTime).Unix()),
			"priority":             fmt.Sprintf("%d", task.Priority),
			"title":                task.Name,
			"hidden":               fmt.Sprintf("%t", task.Hidden),
			"missed_runs":          fmt.Sprintf("%d", task.NumberOfMissedRuns),
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
func (r TaskResult) exitCode() uint32 {
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
		return uint32(r)
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
	Name               string                 `json:"TaskName"`
	Path               string                 `json:"TaskPath"`
	Description        string                 `json:"Description"`
	PSComputerName     string                 `json:"PSComputerName"`
	URI                string                 `json:"URI"`
	Version            string                 `json:"Version"`
	LastRunTime        string                 `json:"LastRunTime"`
	State              TaskState              `json:"State"`
	NextRunTime        string                 `json:"NextRunTime"`
	LastTaskResult     TaskResult             `json:"LastTaskResult"`
	NumberOfMissedRuns int64                  `json:"NumberOfMissedRuns"`
	UserID             string                 `json:"UserId"`
	Enabled            bool                   `json:"Enabled"`
	Priority           int64                  `json:"Priority"`
	Hidden             bool                   `json:"Hidden"`
	ExecutionTimeLimit string                 `json:"ExecutionTimeLimit"`
	Principal          ScheduledTaskPrincipal `json:"Principal"`
	Actions            []ScheduledTaskAction  `json:"Actions"`
	Triggers           []ScheduledTaskTrigger `json:"Triggers"`
	Settings           ScheduledTaskSetting   `json:"Settings"`
}

type ScheduledTaskPrincipal struct {
	DisplayName       string   `json:"DisplayName"`
	ID                string   `json:"Id"`
	GroupID           string   `json:"GroupId"`
	PSComputerName    string   `json:"PSComputerName"`
	RequiredPrivilege []string `json:"RequiredPrivilege"`
	UserID            string   `json:"UserId"`
}

type ScheduledTaskTrigger struct {
	DaysInterval       int64  `json:"DaysInterval"`
	Enabled            bool   `json:"Enabled"`
	EndBoundary        string `json:"EndBoundary"`
	ExecutionTimeLimit string `json:"ExecutionTimeLimit"`
	ID                 string `json:"Id"`
	RandomDelay        string `json:"RandomDelay"`
	Repetition         any    `json:"Repetition"`
	StartBoundary      string `json:"StartBoundary"`
}

type ScheduledTaskSetting struct {
	AllowDemandStart                bool   `json:"AllowDemandStart"`
	AllowHardTerminate              bool   `json:"AllowHardTerminate"`
	DeleteExpiredTaskAfter          string `json:"DeleteExpiredTaskAfter"`
	DisallowStartIfOnBatteries      bool   `json:"DisallowStartIfOnBatteries"`
	DisallowStartOnRemoteAppSession bool   `json:"DisallowStartOnRemoteAppSession"`
	Enabled                         bool   `json:"Enabled"`
	ExecutionTimeLimit              string `json:"ExecutionTimeLimit"`
	Hidden                          bool   `json:"Hidden"`
	IdleSettings                    any    `json:"IdleSettings"`
	MaintenanceSettings             any    `json:"MaintenanceSettings"`
	NetworkSettings                 any    `json:"NetworkSettings"`
	Priority                        int64  `json:"Priority"`
	PSComputerName                  string `json:"PSComputerName"`
	RestartCount                    int64  `json:"RestartCount"`
	RestartInterval                 string `json:"RestartInterval"`
	RunOnlyIfIdle                   bool   `json:"RunOnlyIfIdle"`
	RunOnlyIfNetworkAvailable       bool   `json:"RunOnlyIfNetworkAvailable"`
	StartWhenAvailable              bool   `json:"StartWhenAvailable"`
	StopIfGoingOnBatteries          bool   `json:"StopIfGoingOnBatteries"`
	UseUnifiedSchedulingEngine      bool   `json:"UseUnifiedSchedulingEngine"`
	Volatile                        bool   `json:"Volatile"`
	WakeToRun                       bool   `json:"WakeToRun"`
}

type ScheduledTaskAction struct {
	Arguments        string `json:"Arguments"`
	Execute          string `json:"Execute"`
	ID               string `json:"Id"`
	PSComputerName   string `json:"PSComputerName"`
	WorkingDirectory string `json:"WorkingDirectory"`
}
