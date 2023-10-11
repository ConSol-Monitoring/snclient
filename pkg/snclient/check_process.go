//go:build !windows

package snclient

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"pkg/convert"

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
		hasInventory: ListInventory,
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

	check.ExpandThresholdUnit([]string{"k", "m", "g", "p", "e", "ki", "mi", "gi", "pi", "ei"}, "B", []string{"rss", "virtual", "pagefile"})

	for _, proc := range procs {
		cmdLine, err := proc.Cmdline()
		if err != nil {
			log.Debugf("check_process: cmd line error: %s")
		}

		exe := ""
		filename, err := proc.Exe()
		if err == nil {
			// in case the binary has been removed / updated meanwhile it shows up as "".../path/bin (deleted)""
			// %> ls -la /proc/857375/exe
			// lrwxrwxrwx 1 user group 0 Oct 11 20:40 /proc/857375/exe -> '/usr/bin/ssh (deleted)'
			filename = strings.TrimSuffix(filename, " (deleted)")
			exe = filepath.Base(filename)
		} else {
			cmd, err := proc.CmdlineSlice()
			if err != nil && len(cmd) >= 1 {
				exe = cmd[0]
			}
		}
		if exe == "" {
			name, err := proc.Name()
			if err != nil {
				log.Debugf("check_process: name error: %s")
			} else {
				exe = fmt.Sprintf("[%s]", name)
			}
		}

		if len(l.processes) > 0 && !slices.Contains(l.processes, exe) {
			continue
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

		uids, err := proc.Uids()
		if err != nil {
			log.Debugf("check_process: uids error: %s")
			uids = []int32{-1}
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
			"uid":          fmt.Sprintf("%d", uids[0]),
			"username":     username,
			"virtual":      fmt.Sprintf("%d", mem.VMS),
			"rss":          fmt.Sprintf("%d", mem.RSS),
			"pagefile":     fmt.Sprintf("%d", mem.Swap),
		})
	}

	check.listData = check.Filter(check.filter, check.listData)
	check.result.Metrics = append(check.result.Metrics, &CheckMetric{
		Name:     "count",
		Value:    len(check.listData),
		Min:      &Zero,
		Warning:  check.warnThreshold,
		Critical: check.critThreshold,
	})

	if check.HasThreshold("rss") {
		totalRss := int64(0)
		for _, p := range check.listData {
			val := convert.Float64(p["rss"])
			totalRss += int64(val)
		}
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			Name:     "rss",
			Unit:     "B",
			Value:    totalRss,
			Min:      &Zero,
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
		})
	}

	if check.HasThreshold("virtual") {
		totalVirtual := int64(0)
		for _, p := range check.listData {
			val := convert.Float64(p["virtual"])
			totalVirtual += int64(val)
		}
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			Name:     "virtual",
			Unit:     "B",
			Value:    totalVirtual,
			Min:      &Zero,
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
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
