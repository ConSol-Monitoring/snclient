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
	timeZoneStr string
}

func NewCheckTasksched() CheckHandler {
	return &CheckTasksched{
		timeZoneStr: "Local",
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
			"timezone": {value: &l.timeZoneStr, description: "Sets the timezone for time metrics (default is local time)"},
		},
		defaultFilter:   "enabled = true",
		defaultCritical: "exit_code < 0",
		defaultWarning:  "exit_code != 0",
		detailSyntax:    "${folder}/${title}: ${exit_code} != 0",
		topSyntax:       "%(status) - ${problem_list}",
		okSyntax:        "%(status) - All tasks are ok",
		emptySyntax:     "%(status) - No tasks found",
		emptyState:      CheckExitWarning,
		attributes: []CheckAttribute{
			{name: "application", description: "Name of the application that the task is associated with"},
			{name: "comment", description: "Comment or description for the work item"},
			{name: "creator", description: "Creator of the work item"},
			{name: "enabled", description: "Flag wether this job is enabled (true/false)"},
			{name: "exit_code", description: "The last jobs exit code"},
			{name: "exit_string", description: "The last jobs exit code as string"},
			{name: "folder", description: "Task folder"},
			{name: "max_run_time", description: "Maximum length of time the task can run"},
			{name: "most_recent_run_time", description: "Most recent time the work item began running"},
			{name: "priority", description: "Task priority"},
			{name: "title", description: "Task title"},
			{name: "hidden", description: "Indicates that the task will not be visible in the UI (true/false)"},
			{name: "missed_runs", description: "Number of times the registered task has missed a scheduled run"},
			{name: "task_status", description: "Task status as string"},
			{name: "next_run_time", description: "Time when the registered task is next scheduled to run"},
		},
		exampleDefault: `
    check_tasksched
    OK - All tasks are ok
	`,
		exampleArgs: `'crit=exit_code != 0'`,
	}
}

func (l *CheckTasksched) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	if runtime.GOOS != "windows" {
		// this allows to run make docs on Linux as well, even if it's not in the implemented: attribute
		return nil, fmt.Errorf("check_tasksched is a windows only check")
	}

	err := l.addTasks(check)
	if err != nil {
		return nil, err
	}

	return check.Finalize()
}
