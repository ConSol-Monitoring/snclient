//go:build !windows

package snclient

import (
	"fmt"
	"path/filepath"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_process"] = CheckEntry{"check_process", new(CheckProcess)}
}

type CheckProcess struct {
	noCopy noCopy
}

/* check_process_linux
 * Description: Checks the state of a process on the host.
 */
func (l *CheckProcess) Check(_ *Agent, args []string) (*CheckResult, error) {
	check := &CheckData{
		result: &CheckResult{
			State: CheckExitOK,
		},
		okSyntax:     "%(status): ${list}",
		detailSyntax: "${name}: ${count}",
		topSyntax:    "${status}: ${problem_list}",
		emptyState:   3,
		emptySyntax:  "check_cpu failed to find anything with this filter.",
	}
	argList, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	// parse process filter
	var processes []string
	for _, arg := range argList {
		if arg.key == "process" {
			processes = append(processes, arg.value)
		}
	}

	procs, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("fetching processes failed: %s", err.Error())
	}

	resultProcs := make(map[string]int, 0)
	for _, proc := range procs {
		exe, err := proc.Exe()
		if err != nil {
			continue
		}
		exe = filepath.Base(exe)
		if len(processes) > 0 && !slices.Contains(processes, exe) {
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
