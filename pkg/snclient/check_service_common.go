//go:build windows || linux

package snclient

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/shirou/gopsutil/v4/process"
)

func (l *CheckService) addProcMetrics(ctx context.Context, pidStr string, listEntry map[string]string) error {
	if pidStr == "" {
		return nil
	}
	cpu := float64(0)
	rss := uint64(0)
	vms := uint64(0)
	createTimeUnix := int64(0)
	for _, pidStrEx := range strings.Split(pidStr, ",") {
		pid, err := convert.Int32E(pidStrEx)
		if err != nil {
			return fmt.Errorf("pid is not a number: %s: %s", pidStrEx, err.Error())
		}
		if pid <= 0 {
			return fmt.Errorf("pid is not a positive number: %s", pidStrEx)
		}

		proc, err := process.NewProcess(pid)
		if err != nil {
			log.Tracef("%s", fmt.Errorf("pid not found %d: %s", pid, err.Error()).Error())

			return nil
		}

		cpuP, err := proc.CPUPercentWithContext(ctx)
		if err == nil {
			cpu += cpuP
		}

		mem, _ := proc.MemoryInfoWithContext(ctx)
		if mem != nil {
			rss += mem.RSS
			vms += mem.VMS
		}

		createTimeMillis, err := proc.CreateTimeWithContext(ctx)
		if err == nil {
			ctMillis := createTimeMillis / 1e3
			if createTimeUnix == 0 || ctMillis < createTimeUnix {
				createTimeUnix = ctMillis
			}
		}
	}

	listEntry["cpu"] = fmt.Sprintf("%.1f", cpu)
	listEntry["rss"] = fmt.Sprintf("%d", rss)
	listEntry["vms"] = fmt.Sprintf("%d", vms)
	if createTimeUnix > 0 {
		listEntry["created"] = fmt.Sprintf("%d", createTimeUnix)
		listEntry["age"] = fmt.Sprintf("%d", time.Now().Unix()-createTimeUnix)
	}

	return nil
}

func (l *CheckService) addServiceMetrics(service string, serviceState float64, check *CheckData, listEntry map[string]string) { //nolint:funlen //makes no sense splitting
	if check.HasThreshold("state") {
		check.result.Metrics = append(check.result.Metrics,
			&CheckMetric{
				Name:  service,
				Value: serviceState,
			},
		)
	}

	rssNeeded := check.HasThreshold("rss") || check.showAll || len(l.services) > 0
	vmsNeeded := check.HasThreshold("vms") || check.showAll || len(l.services) > 0
	cpuNeeded := check.HasThreshold("cpu") || check.showAll || len(l.services) > 0
	tasksNeeded := check.HasThreshold("tasks") || check.showAll || len(l.services) > 0

	if rssNeeded || vmsNeeded {
		if _, ok := listEntry["rss"]; ok {
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					ThresholdName: "rss",
					Name:          fmt.Sprintf("%s rss", service),
					Value:         convert.Int64(listEntry["rss"]),
					Unit:          "B",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				},
				&CheckMetric{
					ThresholdName: "vms",
					Name:          fmt.Sprintf("%s vms", service),
					Value:         convert.Int64(listEntry["vms"]),
					Unit:          "B",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				},
			)
		} else {
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					Name:  fmt.Sprintf("%s rss", service),
					Value: "U",
				},
				&CheckMetric{
					Name:  fmt.Sprintf("%s vms", service),
					Value: "U",
				},
			)
		}
	}

	if cpuNeeded {
		if _, ok := listEntry["cpu"]; ok {
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					ThresholdName: "cpu",
					Name:          fmt.Sprintf("%s cpu", service),
					Value:         convert.Float64(listEntry["cpu"]),
					Unit:          "%",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				},
			)
		} else {
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					Name:  fmt.Sprintf("%s cpu", service),
					Value: "U",
				},
			)
		}
	}

	if _, ok := listEntry["tasks"]; ok && tasksNeeded {
		check.result.Metrics = append(check.result.Metrics,
			&CheckMetric{
				ThresholdName: "tasks",
				Name:          fmt.Sprintf("%s tasks", service),
				Value:         convert.Int64(listEntry["tasks"]),
				Unit:          "",
				Warning:       check.warnThreshold,
				Critical:      check.critThreshold,
				Min:           &Zero,
			},
		)
	}
}

func (l *CheckService) isRequired(check *CheckData, entry map[string]string, services, excludes []string) bool {
	name := strings.ToLower(entry["name"])
	desc := strings.ToLower(entry["desc"])
	if slices.Contains(excludes, name) || slices.Contains(excludes, desc) {
		log.Tracef("service %s excluded by exclude list", name)

		return false
	}
	if slices.Contains(services, "*") {
		return true
	}

	foundByName := slices.ContainsFunc(services, func(e string) bool { return strings.EqualFold(e, name) })
	foundByDesc := slices.ContainsFunc(services, func(e string) bool { return strings.EqualFold(e, desc) })

	if len(services) > 0 && !foundByName && !foundByDesc {
		log.Tracef("service %s excluded by not matching service list", name)

		return false
	}

	if !check.MatchMapCondition(check.filter, entry, true) {
		log.Tracef("service %s excluded by filter", name)

		return false
	}

	return true
}
