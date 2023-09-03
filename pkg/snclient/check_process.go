//go:build !windows

package snclient

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_process"] = CheckEntry{"check_process", new(CheckProcess)}
}

type CheckProcess struct {
	processes   []string
	timeZoneStr string
}

func (l *CheckProcess) Build() *CheckData {
	l.processes = []string{}
	l.timeZoneStr = "Local"

	return &CheckData{
		name:         "check_process",
		description:  "Checks the state and metrics of one or multiple processes.",
		hasInventory: true,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]interface{}{
			"process":  &l.processes,
			"timezone": &l.timeZoneStr,
		},
		okSyntax:     "%(status): all processes are ok.",
		detailSyntax: "${exe}=${state}",
		topSyntax:    "${status}: ${problem_list}",
		emptyState:   3,
		emptySyntax:  "check_process failed to find anything with this filter.",
	}
}

func (l *CheckProcess) Check(ctx context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("fetching processes failed: %s", err.Error())
	}
	timeZone, err := time.LoadLocation(l.timeZoneStr)
	if err != nil {
		return nil, fmt.Errorf("couldn't find timezone: %s", l.timeZoneStr)
	}

	for _, proc := range procs {
		filename, err := proc.Exe()
		if err != nil {
			continue
		}
		exe := filepath.Base(filename)
		if len(l.processes) > 0 && !slices.Contains(l.processes, exe) {
			continue
		}

		cmdLine, err := proc.Cmdline()
		if err != nil {
			log.Debugf("check_process: cmd line error: %s")
		}

		states, err := proc.Status()
		if err != nil {
			log.Debugf("check_process: status error: %s")
		}
		state := []string{}
		for _, s := range states {
			state = append(state, convertStatusChar(s))
		}

		ctime, err := proc.CreateTime()
		if err != nil {
			log.Debugf("check_process: CreateTime error: %s")
		}

		username, err := proc.UsernameWithContext(ctx)
		if err != nil {
			log.Debugf("check_process: Username error: %s")
		}

		mem, err := proc.MemoryInfo()
		if err != nil {
			log.Debugf("check_process: Username error: %s")
			mem = &process.MemoryInfoStat{}
		}

		check.listData = append(check.listData, map[string]string{
			"process":      exe,
			"state":        strings.Join(state, ","),
			"command_line": cmdLine,
			"creation":     time.Unix(ctime, 0).In(timeZone).Format("2006-01-02 15:04:05 MST"),
			"exe":          exe,
			"filename":     filename,
			"pid":          fmt.Sprintf("%d", proc.Pid),
			"username":     username,
			"virtual":      fmt.Sprintf("%d", mem.VMS),
			"rss":          fmt.Sprintf("%d", mem.RSS),
			"pagefile":     fmt.Sprintf("%d", mem.Swap),
		})
	}

	return check.Finalize()
}

func convertStatusChar(letter string) string {
	switch strings.ToLower(letter) {
	case "i", "idle":
		return "idle"
	case "l", "lock":
		return "lock"
	case "r", "running":
		return "running"
	case "s", "sleep":
		return "sleep"
	case "t", "stop":
		return "stop"
	case "w", "wait":
		return "wait"
	case "z", "zombie":
		return "zombie"
	default:
		return "unknown"
	}
}
