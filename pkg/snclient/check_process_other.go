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

func (l *CheckProcess) fetchProcs(ctx context.Context, check *CheckData) error {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return fmt.Errorf("fetching processes failed: %s", err.Error())
	}
	timeZone, err := time.LoadLocation(l.timeZoneStr)
	if err != nil {
		return fmt.Errorf("couldn't find timezone: %s", l.timeZoneStr)
	}

	userNameLookup := map[int32]string{}

	for _, proc := range procs {
		cmdLine, err := proc.CmdlineWithContext(ctx)
		if err != nil {
			log.Debugf("check_process: cmd line error: %s", err.Error())
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
			cmd, err2 := proc.CmdlineSliceWithContext(ctx)
			if err2 != nil && len(cmd) >= 1 {
				exe = cmd[0]
			}
		}
		if exe == "" {
			name, err2 := proc.NameWithContext(ctx)
			if err2 != nil {
				log.Debugf("check_process: name error: %s", err2.Error())
			} else {
				exe = fmt.Sprintf("[%s]", name)
			}
		}

		if len(l.processes) > 0 && !slices.Contains(l.processes, exe) && !slices.Contains(l.processes, "*") {
			continue
		}

		states, err := proc.StatusWithContext(ctx)
		if err != nil {
			log.Debugf("check_process: status error: %s", err.Error())
		}
		state := []string{}
		for _, s := range states {
			state = append(state, convertStatusChar(s))
		}

		ctime, err := proc.CreateTimeWithContext(ctx)
		if err != nil {
			log.Debugf("check_process: CreateTime error: %s", err.Error())
		}

		// skip very young ( < 3000ms ) check_nsc_web processes, they might be checking us and screwing process counts
		if strings.Contains(cmdLine, "check_nsc_web") && time.Now().UnixMilli()-ctime < 3000 {
			continue
		}

		uids, err := proc.UidsWithContext(ctx)
		if err != nil {
			log.Debugf("check_process: uids error: %s", err.Error())
			uids = []int32{-1}
		}

		// cache user name lookups
		username := userNameLookup[uids[0]]
		if username == "" {
			username, err = proc.UsernameWithContext(ctx)
			if err != nil {
				log.Debugf("check_process: Username error uid %#v: %s", uids, err.Error())
			}
			userNameLookup[uids[0]] = username
		}

		mem, err := proc.MemoryInfoWithContext(ctx)
		if err != nil {
			log.Debugf("check_process: meminfo error: %s", err.Error())
			mem = &process.MemoryInfoStat{}
		}

		cpu, err := proc.CPUPercentWithContext(ctx)
		if err != nil {
			log.Debugf("check_process: cpuinfo error: %s", err.Error())
		}

		check.listData = append(check.listData, map[string]string{
			"process":       exe,
			"state":         strings.Join(state, ","),
			"command_line":  cmdLine,
			"creation":      time.UnixMilli(ctime).In(timeZone).Format("2006-01-02 15:04:05 MST"),
			"creation_unix": fmt.Sprintf("%d", time.UnixMilli(ctime).Unix()),
			"exe":           exe,
			"filename":      filename,
			"pid":           fmt.Sprintf("%d", proc.Pid),
			"uid":           fmt.Sprintf("%d", uids[0]),
			"username":      username,
			"virtual":       fmt.Sprintf("%d", mem.VMS),
			"rss":           fmt.Sprintf("%d", mem.RSS),
			"pagefile":      fmt.Sprintf("%d", mem.Swap),
			"cpu":           fmt.Sprintf("%f", cpu),
		})
	}

	return nil
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
