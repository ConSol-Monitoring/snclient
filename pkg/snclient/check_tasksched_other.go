//go:build !windows

package snclient

import (
	"context"
	"fmt"
)

func (l *CheckTasksched) addTasks(_ context.Context, _ *Agent, _ *CheckData) error {
	return fmt.Errorf("check_tasksched is a windows only check")
}
