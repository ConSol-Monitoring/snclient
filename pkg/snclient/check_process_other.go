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

func (l *CheckProcess) Check(ctx context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching processes failed: %s", err.Error())
	}
	timeZone, err := time.LoadLocation(l.timeZoneStr)
	if err != nil {
		return nil, fmt.Errorf("couldn't find timezone: %s", l.timeZoneStr)
	}

	check.ExpandThresholdUnit([]string{"k", "m", "g", "p", "e", "ki", "mi", "gi", "pi", "ei"}, "B", []string{"rss", "virtual", "pagefile"})

	userNameLookup := map[int32]string{}

	for _, proc := range procs {
		cmdLine, err := proc.CmdlineWithContext(ctx)
		if err != nil {
			log.Debugf("check_process: cmd line error: %s")
		}

		exe := ""
		filename, err := proc.ExeWithContext(ctx)
		if err == nil {
			// in case the binary has been removed / updated meanwhile it shows up as "".../path/bin (deleted)""
			// %> ls -la /proc/857375/exe
			// lrwxrwxrwx 1 user group 0 Oct 11 20:40 /proc/857375/exe -> '/usr/bin/ssh (deleted)'
			filename = strings.TrimSuffix(filename, " (deleted)")
			exe = filepath.Base(filename)
		} else {
			cmd, err := proc.CmdlineSliceWithContext(ctx)
			if err != nil && len(cmd) >= 1 {
				exe = cmd[0]
			}
		}
		if exe == "" {
			name, err := proc.NameWithContext(ctx)
			if err != nil {
				log.Debugf("check_process: name error: %s")
			} else {
				exe = fmt.Sprintf("[%s]", name)
			}
		}

		if len(l.processes) > 0 && !slices.Contains(l.processes, exe) {
			continue
		}

		states, err := proc.StatusWithContext(ctx)
		if err != nil {
			log.Debugf("check_process: status error: %s")
		}
		state := []string{}
		for _, s := range states {
			state = append(state, convertStatusChar(s))
		}

		ctime, err := proc.CreateTimeWithContext(ctx)
		if err != nil {
			log.Debugf("check_process: CreateTime error: %s")
		}

		uids, err := proc.UidsWithContext(ctx)
		if err != nil {
			log.Debugf("check_process: uids error: %s")
			uids = []int32{-1}
		}

		// cache user name lookups
		username := userNameLookup[uids[0]]
		if username == "" {
			username, err := proc.UsernameWithContext(ctx)
			if err != nil {
				log.Debugf("check_process: Username error: %s")
			}
			userNameLookup[uids[0]] = username
		}

		mem, err := proc.MemoryInfoWithContext(ctx)
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
