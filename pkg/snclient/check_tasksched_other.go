//go:build !windows

package snclient

import (
	"fmt"
)

func (l *CheckTasksched) addTasks(_ *CheckData) error {
	return fmt.Errorf("check_tasksched is a windows only check")
}
