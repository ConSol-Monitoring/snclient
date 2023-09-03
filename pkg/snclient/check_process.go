//go:build !windows

package snclient

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_process"] = CheckEntry{"check_process", new(CheckProcess)}
}

type CheckProcess struct {
	processes []string
}

func (l *CheckProcess) Build() *CheckData {
	l.processes = []string{}

	return &CheckData{
		name:         "check_process",
		description:  "Checks the state and metrics of one or multiple processes.",
		hasInventory: true,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]interface{}{
			"process": &l.processes,
		},
		okSyntax:     "%(status): ${list}",
		detailSyntax: "${name}: ${count}",
		topSyntax:    "${status}: ${problem_list}",
		emptyState:   3,
		emptySyntax:  "check_process failed to find anything with this filter.",
	}
}

func (l *CheckProcess) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("fetching processes failed: %s", err.Error())
	}
	// TODO: ... make like windows

	resultProcs := make(map[string]int, 0)
	for _, proc := range procs {
		exe, err := proc.Exe()
		if err != nil {
			continue
		}
		exe = filepath.Base(exe)
		if len(l.processes) > 0 && !slices.Contains(l.processes, exe) {
			continue
		}

		_, ok := resultProcs[exe]
		if !ok {
			resultProcs[exe] = 0
		}
		resultProcs[exe]++
	}

	for exe, count := range resultProcs {
		check.listData = append(check.listData, map[string]string{
			"name":  exe,
			"count": fmt.Sprintf("%d", count),
		})
	}

	return check.Finalize()
}
