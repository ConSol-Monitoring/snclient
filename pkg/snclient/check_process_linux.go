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
	data   CheckData
}

/* check_process_linux
 * Description: Checks the state of a process on the host.
 */
func (l *CheckProcess) Check(_ *Agent, args []string) (*CheckResult, error) {
	metrics := make([]*CheckMetric, 0)
	argList, err := ParseArgs(args, &l.data)
	if err != nil {
		return nil, fmt.Errorf("args error: %s", err.Error())
	}

	var processes []string

	// parse process filter
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
		metrics = append(metrics, &CheckMetric{
			Name:  exe,
			Value: float64(count),
		})
	}

	return &CheckResult{
		State:   0,
		Output:  "",
		Metrics: metrics,
	}, nil
}
