//go:build windows

package snclient

import (
	"context"
	"fmt"
	"time"

	"github.com/capnspacehook/taskmaster"
)

func init() {
	AvailableChecks["check_tasksched"] = CheckEntry{"check_tasksched", new(CheckTasksched)}
}

type CheckTasksched struct {
	timeZoneStr string
}

func (l *CheckTasksched) Build() *CheckData {
	l.timeZoneStr = "Local"

	return &CheckData{
		name:        "check_tasksched",
		description: "checks scheduled tasks",
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]interface{}{
			"timezone": &l.timeZoneStr,
		},
		defaultFilter:   "enabled = true",
		defaultCritical: "exit_code < 0",
		defaultWarning:  "exit_code != 0",
		detailSyntax:    "${folder}/${title}: ${exit_code} != 0",
		topSyntax:       "${status}: ${problem_list}",
		okSyntax:        "%(status): All tasks are ok",
		emptySyntax:     "%(status): No tasks found",
		emptyState:      CheckExitWarning,
	}
}

func (l *CheckTasksched) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	timeZone, err := time.LoadLocation(l.timeZoneStr)
	if err != nil {
		return nil, fmt.Errorf("couldn't find timezone: %s", l.timeZoneStr)
	}

	// connect to task scheduler
	taskSvc, err := taskmaster.Connect()
	if err != nil {
		return &CheckResult{
			State:  int64(3),
			Output: fmt.Sprintf("Failed to open task scheduler: %s", err),
		}, nil
	}

	taskList, err := taskSvc.GetRegisteredTasks()
	if err != nil {
		return &CheckResult{
			State:  int64(3),
			Output: fmt.Sprintf("Failed to fetch scheduled task list: %s", err),
		}, nil
	}

	for index := range taskList {
		entry := map[string]string{
			"application":          taskList[index].Name,
			"comment":              taskList[index].Definition.RegistrationInfo.Description,
			"creator":              taskList[index].Definition.RegistrationInfo.Author,
			"enabled":              fmt.Sprintf("%t", taskList[index].Enabled),
			"exit_code":            l.taskExitCode(taskList[index].LastTaskResult),
			"exit_string":          taskList[index].LastTaskResult.String(),
			"folder":               taskList[index].Path,
			"max_run_time":         taskList[index].Definition.Settings.TimeLimit.String(),
			"most_recent_run_time": taskList[index].LastRunTime.In(timeZone).Format("2006-01-02 15:04:05 MST"),
			"priority":             fmt.Sprintf("%d", taskList[index].Definition.Settings.Priority),
			"title":                taskList[index].Name,
			"hidden":               fmt.Sprintf("%t", taskList[index].Definition.Settings.Hidden),
			"missed_runs":          fmt.Sprintf("%d", taskList[index].MissedRuns),
			"task_status":          taskList[index].State.String(),
			"next_run_time":        taskList[index].NextRunTime.In(timeZone).Format("2006-01-02 15:04:05 MST"),
		}
		check.listData = append(check.listData, entry)
	}

	return check.Finalize()
}

func (l *CheckTasksched) taskExitCode(taskResult taskmaster.TaskResult) string {
	switch taskResult {
	case taskmaster.SCHED_S_SUCCESS:
		return "0"
	case taskmaster.SCHED_S_TASK_READY:
		return "1"
	case taskmaster.SCHED_S_TASK_RUNNING:
		return "2"
	case taskmaster.SCHED_S_TASK_DISABLED:
		return "3"
	case taskmaster.SCHED_S_TASK_HAS_NOT_RUN:
		return "4"
	case taskmaster.SCHED_S_TASK_NO_MORE_RUNS:
		return "5"
	case taskmaster.SCHED_S_TASK_NOT_SCHEDULED:
		return "6"
	case taskmaster.SCHED_S_TASK_TERMINATED:
		return "7"
	case taskmaster.SCHED_S_TASK_NO_VALID_TRIGGERS:
		return "8"
	case taskmaster.SCHED_S_EVENT_TRIGGER:
		return "9"
	case taskmaster.SCHED_S_SOME_TRIGGERS_FAILED:
		return "10"
	case taskmaster.SCHED_S_BATCH_LOGON_PROBLEM:
		return "11"
	case taskmaster.SCHED_S_TASK_QUEUED:
		return "12"
	default:
		return "-1"
	}
}
