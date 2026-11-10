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
		description: "Check the status of scheduled tasks.",
		implemented: Windows,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"timezone": {description: "Set the timezone for time metrics (default is local time)"},
			"title": {
				value:       &l.TaskTitle,
				description: fmt.Sprintf("Title of the scheduled task to check. Default: '%s'", CheckTaskschedDefaultTaskTitle),
			},
			"folder": {
				value: &l.Folder,
				description: fmt.Sprintf("Folder that contains the scheduled task. Matches this folder exactly unless recursive is enabled. Default: '%s'",
					CheckTaskschedDefaultFolder),
			},
			"recursive": {
				value:       &l.Recursive,
				description: fmt.Sprintf("Search subfolders of the specified folder for scheduled tasks. Default: '%t'", CheckTaskschedDefaultRecursive),
			},
			"hidden": {
				value:       &l.Hidden,
				description: fmt.Sprintf("Include hidden tasks. Default: '%t'", CheckTaskschedDefaultHidden),
			},
		},
		defaultFilter:   "enabled = true",
		defaultCritical: "exit_code < 0",
		defaultWarning:  "exit_code != 0",
		detailSyntax:    "%(uri_clean) (%(most_recent_run_time:date)) exited with %(exit_code)",
		topSyntax:       "%(status) - %(problem_list)",
		okSyntax:        "%(status) - All tasks are ok",
		emptySyntax:     "%(status) - No tasks found",
		emptyState:      CheckExitUnknown,
		attributes: []CheckAttribute{
			{name: "application", description: "Name of the application that the task is associated with"},
			{name: "comment", description: "Comment or description for the work item"},
			{name: "creator", description: "Creator of the work item"},
			{name: "enabled", description: "Flag whether this job is enabled (true/false)", unit: UBool},
			{name: "exit_code", description: "The last job's exit code"},
			{name: "exit_string", description: "The last job's exit code as string"},
			{name: "folder", description: "Task folder"},
			{name: "uri", description: "Full path to the task, including the folder and task title"},
			{name: "uri_clean", description: "URI without the leading backslash for tasks directly at root. Tasks inside folders keep their leading backslash."},
			{name: "has_run", description: "True if this task has ever been executed", unit: UBool},
			{name: "max_run_time", description: "Maximum length of time the task can run", unit: UDuration},
			{name: "most_recent_run_time", description: "Most recent time the work item began running", unit: UDate},
			{name: "priority", description: "Task priority"},
			{name: "title", description: "Task title"},
			{name: "hidden", description: "True if the task is hidden in the UI (true/false)", unit: UBool},
			{name: "missed_runs", description: "Number of scheduled runs that the registered task missed"},
			{name: "task_status", description: "Task status as string"},
			{name: "next_run_time", description: "Time when the registered task is next scheduled to run", unit: UDate},
			{name: "parameters", description: "Last action's command line parameters"},
			{name: "execute", description: "Last action's executed program"},
			{name: "working_directory", description: "Last action's working directory"},
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
