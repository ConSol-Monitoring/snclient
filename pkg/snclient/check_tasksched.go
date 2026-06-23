package snclient

import (
	"context"
	"fmt"
	"runtime"
)

func init() {
	AvailableChecks["check_tasksched"] = CheckEntry{"check_tasksched", NewCheckTasksched}
}

type CheckTasksched struct {
	TaskTitle string
	Folder    string
	Recursive bool
	Hidden    bool
}

const (
	CheckTaskschedDefaultTaskTitle string = "*"
	CheckTaskschedDefaultFolder    string = "\\"
	CheckTaskschedDefaultRecursive bool   = true
	CheckTaskschedDefaultHidden    bool   = false
)

func NewCheckTasksched() CheckHandler {
	return &CheckTasksched{
		TaskTitle: CheckTaskschedDefaultTaskTitle,
		Folder:    CheckTaskschedDefaultFolder,
		Recursive: CheckTaskschedDefaultRecursive,
		Hidden:    CheckTaskschedDefaultHidden,
	}
}

func (l *CheckTasksched) Build() *CheckData {
	return &CheckData{
		name:        "check_tasksched",
		description: "Check status of scheduled jobs",
		implemented: Windows,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"timezone":  {description: "Sets the timezone for time metrics (default is local time)"},
			"title":     {value: &l.TaskTitle, description: "Sets the task to check. This corresonds to the title of the scheduled task, called TaskName in Powershell output."},
			"folder":    {value: &l.Folder, description: "The folder where the scheduled task is saved. This is used for exact matches, unless recurisive option is enabled."},
			"recursive": {value: &l.Recursive, description: "Include the subfolders of the specified folder as well when searching for scheduled tasks."},
			"hidden":    {value: &l.Hidden, description: "Include hidden tasks."},
		},
		defaultFilter:   "enabled = true",
		defaultCritical: "exit_code < 0",
		defaultWarning:  "exit_code != 0",
		detailSyntax:    "${uri_clean} (%{most_recent_run_time:date}) exited with ${exit_code}",
		topSyntax:       "%(status) - ${problem_list}",
		okSyntax:        "%(status) - All tasks are ok",
		emptySyntax:     "%(status) - No tasks found",
		emptyState:      CheckExitUnknown,
		attributes: []CheckAttribute{
			{name: "application", description: "Name of the application that the task is associated with"},
			{name: "comment", description: "Comment or description for the work item"},
			{name: "creator", description: "Creator of the work item"},
			{name: "enabled", description: "Flag whether this job is enabled (true/false)", unit: UBool},
			{name: "exit_code", description: "The last jobs exit code"},
			{name: "exit_string", description: "The last jobs exit code as string"},
			{name: "folder", description: "Task folder"},
			{name: "uri", description: "Fully qualified path to the task, includes folder and the task title"},
			{name: "uri_clean", description: "Remove the leading backslash from the URI, only for tasks directly saved at root and not for ones saved inside folders."},
			{name: "has_run", description: "True if this task has ever been executed", unit: UBool},
			{name: "max_run_time", description: "Maximum length of time the task can run", unit: UDuration},
			{name: "most_recent_run_time", description: "Most recent time the work item began running", unit: UDate},
			{name: "priority", description: "Task priority"},
			{name: "title", description: "Task title"},
			{name: "hidden", description: "Indicates that the task will not be visible in the UI (true/false)", unit: UBool},
			{name: "missed_runs", description: "Number of times the registered task has missed a scheduled run"},
			{name: "task_status", description: "Task status as string"},
			{name: "next_run_time", description: "Time when the registered task is next scheduled to run", unit: UDate},
			{name: "parameters", description: "Last actions command line parameters"},
			{name: "execute", description: "Last actions executed program"},
			{name: "working_directory", description: "Last actions working directory"},
		},
		exampleDefault: `
    check_tasksched
    OK - All tasks are ok
	`,
		exampleArgs: `'crit=exit_code != 0'`,
	}
}

func (l *CheckTasksched) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	if runtime.GOOS != "windows" {
		// this allows to run make docs on Linux as well, even if it's not in the implemented: attribute
		return nil, fmt.Errorf("check_tasksched is a windows only check")
	}

	err := l.addTasks(ctx, snc, check)
	if err != nil {
		return nil, err
	}

	return check.Finalize()
}
