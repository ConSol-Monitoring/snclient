package snclient

import (
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

// cpuTimesTotal returns the sum of all time fields of a CPU times stat.
func cpuTimesTotal(cpuT *cpu.TimesStat) float64 {
	return cpuT.User + cpuT.System + cpuT.Idle + cpuT.Nice + cpuT.Iowait + cpuT.Irq + cpuT.Softirq + cpuT.Steal + cpuT.Guest + cpuT.GuestNice
}

// cpuSampleStore keeps at most two CPU samples per process (identified by pid and
// creation time): a baseline (newest sample at least one minute old) and the latest sample.
type cpuSampleStore struct {
	mu        sync.Mutex
	samples   map[cpuSampleKey]*cpuSampleEntry
	lastSweep time.Time
}

type cpuSampleKey struct {
	pid     int32
	created int64
}

type cpuSample struct {
	cpuTime float64
	at      time.Time
}

type cpuSampleEntry struct {
	baseline *cpuSample
	latest   *cpuSample
}

const (
	cpuSampleMinAge = time.Minute
	cpuSampleMaxAge = time.Hour
)

func newCPUSampleStore() *cpuSampleStore {
	return &cpuSampleStore{samples: make(map[cpuSampleKey]*cpuSampleEntry)}
}

// update registers the current CPU time of a process and returns its usage in percent
// since the newest sample at least one minute old, or ok=false if no such sample exists.
func (s *cpuSampleStore) update(pid int32, created int64, cpuTime float64, now time.Time) (float64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if now.Sub(s.lastSweep) >= cpuSampleMinAge {
		s.lastSweep = now
		for k, e := range s.samples {
			if e.latest != nil && now.Sub(e.latest.at) > cpuSampleMaxAge {
				delete(s.samples, k)
			}
		}
	}

	key := cpuSampleKey{pid: pid, created: created}
	entry, ok := s.samples[key]
	if !ok {
		entry = &cpuSampleEntry{}
		s.samples[key] = entry
	}

	// keep at most one sample per minute: only replace a sample that is at least one minute old
	replace := entry.latest == nil || now.Sub(entry.latest.at) >= cpuSampleMinAge
	if replace && entry.latest != nil {
		if now.Sub(entry.latest.at) > cpuSampleMaxAge {
			entry.baseline = nil
		} else {
			entry.baseline = entry.latest
		}
	}

	var (
		pct float64
		got bool
	)
	if entry.baseline != nil && now.Sub(entry.baseline.at) <= cpuSampleMaxAge && cpuTime >= entry.baseline.cpuTime {
		pct = 100 * (cpuTime - entry.baseline.cpuTime) / now.Sub(entry.baseline.at).Seconds()
		got = true
	}

	if replace {
		entry.latest = &cpuSample{cpuTime: cpuTime, at: now}
	}

	return pct, got
}

// cpuUsageSince is a nil-safe wrapper around the agent's CPU sample store.
func (snc *Agent) cpuUsageSince(pid int32, created int64, cpuTime float64, now time.Time) (float64, bool) {
	if snc == nil || snc.procCPU == nil {
		return 0, false
	}

	return snc.procCPU.update(pid, created, cpuTime, now)
}
