//go:build !windows

package snclient

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

func (l *CheckProcess) fetchProcs(ctx context.Context, check *CheckData) error {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return fmt.Errorf("fetching processes failed: %s", err.Error())
	}

	userNameLookup := map[uint32]string{}

	for _, proc := range procs {
		cmdLine, err := proc.CmdlineWithContext(ctx)
		if err != nil {
			log.Debugf("check_process: cmd line error: %s", err.Error())
		}

		exe, filename := buildExeAndFilename(ctx, proc)

		if len(l.processes) > 0 && !slices.Contains(l.processes, strings.ToLower(exe)) && !slices.Contains(l.processes, "*") {
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

		ctimeMilli, err := proc.CreateTimeWithContext(ctx)
		if err != nil {
			log.Debugf("check_process: CreateTime error: %s", err.Error())
		}

		// skip very young ( < 3000ms ) check_nsc_web processes, they might be checking us and screwing process counts
		if strings.Contains(cmdLine, "check_nsc_web") && time.Now().UnixMilli()-ctimeMilli < 3000 {
			continue
		}

		username := ""
		uid := -1
		uids, err := proc.UidsWithContext(ctx)
		if err != nil {
			log.Debugf("check_process: uids error: %s", err.Error())
		} else if len(uids) > 0 {
			// cache user name lookups
			uid = int(uids[0])
			username = userNameLookup[uids[0]]
			if username == "" {
				username, err = proc.UsernameWithContext(ctx)
				if err != nil {
					log.Debugf("check_process: Username error uid %#v: %s", uids, err.Error())
				}
				userNameLookup[uids[0]] = username
			}
		}

		// process does not exist anymore
		if exe == "" && uid == -1 {
			if ok, _ := process.PidExistsWithContext(ctx, proc.Pid); !ok {
				continue
			}
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

		cpuSeconds := float64(0)
		cpuT, err := proc.TimesWithContext(ctx)
		if err != nil {
			log.Debugf("check_process: cpuinfo error: %s", err.Error())
		} else {
			cpuSeconds = cpuT.User + cpuT.System + cpuT.Idle + cpuT.Nice + cpuT.Iowait + cpuT.Irq + cpuT.Softirq + cpuT.Steal + cpuT.Guest + cpuT.GuestNice
		}

		check.listData = append(check.listData, map[string]string{
			"process":      exe,
			"state":        strings.Join(state, ","),
			"command_line": cmdLine,
			"creation":     fmt.Sprintf("%d", ctimeMilli/1000),
			"exe":          exe,
			"filename":     filename,
			"pid":          fmt.Sprintf("%d", proc.Pid),
			"uid":          fmt.Sprintf("%d", uid),
			"username":     username,
			"virtual":      fmt.Sprintf("%d", mem.VMS),
			"rss":          fmt.Sprintf("%d", mem.RSS),
			"pagefile":     fmt.Sprintf("%d", mem.Swap),
			"cpu":          fmt.Sprintf("%f", cpu),
			"cpu_seconds":  fmt.Sprintf("%f", cpuSeconds),
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
	case "b", "blocked":
		return "blocked"
	default:
		return "unknown"
	}
}
