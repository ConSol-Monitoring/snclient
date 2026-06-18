//go:build windows

package snclient

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/goccy/go-json"
)

//go:embed embed/scripts/windows/scheduled_tasks.ps1
var scheduledTasksPS1 string

//nolint:funlen // function is long, but is simple, should not be dismantled
func (l *CheckTasksched) addTasks(ctx context.Context, snc *Agent, check *CheckData) error {
	script := scheduledTasksPS1

	// Add backslash to the beginning of the folder path if it does not exist
	if l.Folder != CheckTaskschedDefaultFolder {
		if !strings.HasPrefix(l.Folder, "\\") {
			l.Folder = "\\" + l.Folder
		}
	}

	if l.TaskTitle != CheckTaskschedDefaultTaskTitle {
		if strings.ContainsFunc(l.TaskTitle, func(r rune) bool { return !unicode.IsLetter(r) }) {
			return fmt.Errorf("custom specified title should be all letters, but it isnt: %s", l.TaskTitle)
		}
	}

	if l.Folder != CheckTaskschedDefaultFolder {
		if strings.ContainsFunc(l.Folder, func(r rune) bool { return !unicode.IsLetter(r) && r != '\\' }) {
			return fmt.Errorf("custom specified folder should be all letters or backslashes, but it isnt: %s", l.Folder)
		}
	}

	cmd, err := powerShellCmd(
		ctx, script,
		PowerShellParameter{
			name:                "title",
			parameterType:       "string",
			specifyDefaultValue: true,
			defaultValue:        CheckTaskschedDefaultTaskTitle,
			specifyValue:        true,
			specifiedValue:      l.TaskTitle,
		},
		PowerShellParameter{
			name:                "folder",
			parameterType:       "string",
			specifyDefaultValue: true,
			defaultValue:        CheckTaskschedDefaultFolder,
			specifyValue:        true,
			specifiedValue:      l.Folder,
		},
		PowerShellParameter{
			name:                "recursive",
			parameterType:       "string",
			specifyDefaultValue: true,
			defaultValue:        strconv.FormatBool(CheckTaskschedDefaultRecursive),
			specifyValue:        true,
			specifiedValue:      strconv.FormatBool(l.Recursive),
		},
	)

	if err != nil {
		return fmt.Errorf("error when building a powershell command: %s", err.Error())
	}

	output, stderr, exitCode, _, err := snc.runExternalCommand(ctx, cmd, snc.getBuiltinCmdTimeout())
	if err != nil {
		return fmt.Errorf("getting scheduled tasks failed, error: %s\n%s", err.Error(), stderr)
	}
	if exitCode != 0 {
		return fmt.Errorf("getting scheduled tasks failed, exitCode: %d, output: %s\n%s", exitCode, output, stderr)
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

		entry := map[string]string{
			"application":          task.Name,
			"comment":              task.Description,
			"creator":              task.UserID,
			"enabled":              fmt.Sprintf("%t", task.Enabled),
			"exit_code":            fmt.Sprintf("%d", task.LastTaskResult.exitCode()),
			"exit_string":          task.LastTaskResult.String(),
			"folder":               task.Path,
			"uri":                  task.URI,
			"uri_clean":            parseURIClean(task.URI),
			"has_run":              fmt.Sprintf("%t", hasRun),
			"max_run_time":         task.ExecutionTimeLimit,
			"most_recent_run_time": fmt.Sprintf("%d", l.parseDate(task.LastRunTime).Unix()),
			"priority":             fmt.Sprintf("%d", task.Priority),
			"title":                task.Name,
			"hidden":               fmt.Sprintf("%t", task.Hidden),
			"missed_runs":          fmt.Sprintf("%d", task.NumberOfMissedRuns),
			"task_status":          task.State.String(),
			"next_run_time":        fmt.Sprintf("%d", l.parseDate(task.NextRunTime).Unix()),
			"parameters":           l.parseParameters(task.Actions),
			"execute":              l.parseExecuteCmd(task.Actions),
			"working_dir":          l.parseWorkingDir(task.Actions),
		}
		check.listData = append(check.listData, entry)
	}

	if check.HasThreshold("title") || check.hasThresholdCond(check.filter, "title") || l.TaskTitle != CheckTaskschedDefaultTaskTitle {
		check.emptyState = CheckExitUnknown
		check.emptySyntax = "%(status) - No tasks found, check your arguments/filters/thresholds using title attribute."
	}

	return nil
}

func parseURIClean(uri string) string {
	if strings.Count(uri, "\\") == 1 {
		if cut, cutOk := strings.CutPrefix(uri, "\\"); cutOk {
			return cut
		}
	}

	return uri
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

func (l *CheckTasksched) parseParameters(actions []ScheduledTaskAction) string {
	if len(actions) == 0 {
		return ""
	}

	return actions[len(actions)-1].Arguments
}

func (l *CheckTasksched) parseExecuteCmd(actions []ScheduledTaskAction) string {
	if len(actions) == 0 {
		return ""
	}

	return actions[len(actions)-1].Execute
}

func (l *CheckTasksched) parseWorkingDir(actions []ScheduledTaskAction) string {
	if len(actions) == 0 {
		return ""
	}

	return actions[len(actions)-1].WorkingDirectory
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

// The script does not export everything it discovers to JSON for snclient to parse
// When needed, modify the script and uncomment these lines

type ScheduledTask struct {
	Name               string                `json:"TaskName"`
	Path               string                `json:"TaskPath"`
	Description        string                `json:"Description"`
	PSComputerName     string                `json:"PSComputerName"`
	URI                string                `json:"URI"`
	Version            string                `json:"Version"`
	LastRunTime        string                `json:"LastRunTime"`
	State              TaskState             `json:"State"`
	NextRunTime        string                `json:"NextRunTime"`
	LastTaskResult     TaskResult            `json:"LastTaskResult"`
	NumberOfMissedRuns int64                 `json:"NumberOfMissedRuns"`
	UserID             string                `json:"UserId"`
	Enabled            bool                  `json:"Enabled"`
	Priority           int64                 `json:"Priority"`
	Hidden             bool                  `json:"Hidden"`
	ExecutionTimeLimit string                `json:"ExecutionTimeLimit"`
	Actions            []ScheduledTaskAction `json:"Actions"`
	// Principal          ScheduledTaskPrincipal `json:"Principal"`
	// Triggers           []ScheduledTaskTrigger `json:"Triggers"`
	// Settings           ScheduledTaskSetting   `json:"Settings"`
}

type ScheduledTaskAction struct {
	Arguments        string `json:"Arguments"`
	Execute          string `json:"Execute"`
	ID               string `json:"Id"`
	PSComputerName   string `json:"PSComputerName"`
	WorkingDirectory string `json:"WorkingDirectory"`
}

// The script does not export everything it discovers to JSON for snclient to parse
// When needed, modify the script and uncomment these lines
// type ScheduledTaskPrincipal struct {
// 	DisplayName       string   `json:"DisplayName"`
// 	ID                string   `json:"Id"`
// 	GroupID           string   `json:"GroupId"`
// 	PSComputerName    string   `json:"PSComputerName"`
// 	RequiredPrivilege []string `json:"RequiredPrivilege"`
// 	UserID            string   `json:"UserId"`
// }

// The script does not export everything it discovers to JSON for snclient to parse
// When needed, modify the script and uncomment these lines
// type ScheduledTaskTrigger struct {
// 	DaysInterval       int64  `json:"DaysInterval"`
// 	Enabled            bool   `json:"Enabled"`
// 	EndBoundary        string `json:"EndBoundary"`
// 	ExecutionTimeLimit string `json:"ExecutionTimeLimit"`
// 	ID                 string `json:"Id"`
// 	RandomDelay        string `json:"RandomDelay"`
// 	Repetition         any    `json:"Repetition"`
// 	StartBoundary      string `json:"StartBoundary"`
// }

// The script does not export everything it discovers to JSON for snclient to parse
// When needed, modify the script and uncomment these lines
// type ScheduledTaskSetting struct {
// 	AllowDemandStart                bool   `json:"AllowDemandStart"`
// 	AllowHardTerminate              bool   `json:"AllowHardTerminate"`
// 	DeleteExpiredTaskAfter          string `json:"DeleteExpiredTaskAfter"`
// 	DisallowStartIfOnBatteries      bool   `json:"DisallowStartIfOnBatteries"`
// 	DisallowStartOnRemoteAppSession bool   `json:"DisallowStartOnRemoteAppSession"`
// 	Enabled                         bool   `json:"Enabled"`
// 	ExecutionTimeLimit              string `json:"ExecutionTimeLimit"`
// 	Hidden                          bool   `json:"Hidden"`
// 	IdleSettings                    any    `json:"IdleSettings"`
// 	MaintenanceSettings             any    `json:"MaintenanceSettings"`
// 	NetworkSettings                 any    `json:"NetworkSettings"`
// 	Priority                        int64  `json:"Priority"`
// 	PSComputerName                  string `json:"PSComputerName"`
// 	RestartCount                    int64  `json:"RestartCount"`
// 	RestartInterval                 string `json:"RestartInterval"`
// 	RunOnlyIfIdle                   bool   `json:"RunOnlyIfIdle"`
// 	RunOnlyIfNetworkAvailable       bool   `json:"RunOnlyIfNetworkAvailable"`
// 	StartWhenAvailable              bool   `json:"StartWhenAvailable"`
// 	StopIfGoingOnBatteries          bool   `json:"StopIfGoingOnBatteries"`
// 	UseUnifiedSchedulingEngine      bool   `json:"UseUnifiedSchedulingEngine"`
// 	Volatile                        bool   `json:"Volatile"`
// 	WakeToRun                       bool   `json:"WakeToRun"`
// }
