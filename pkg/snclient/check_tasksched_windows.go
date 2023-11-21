//go:build windows

package snclient

import (
	"fmt"
	"time"

	"github.com/capnspacehook/taskmaster"
)

func (l *CheckTasksched) addTasks(check *CheckData) error {
	timeZone, err := time.LoadLocation(l.timeZoneStr)
	if err != nil {
		return fmt.Errorf("couldn't find timezone: %s", l.timeZoneStr)
	}

	// connect to task scheduler
	taskSvc, err := taskmaster.Connect()
	if err != nil {
		return fmt.Errorf("failed to open task scheduler: %s", err.Error())
	}

	taskList, err := taskSvc.GetRegisteredTasks()
	if err != nil {
		return fmt.Errorf("failed to fetch scheduled task list: %s", err.Error())
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

	return nil
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
