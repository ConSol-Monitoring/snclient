//go:build !windows

package wincpu

import (
	"time"

	cpuinfo "github.com/shirou/gopsutil/v4/cpu"
)

func Percent(interval time.Duration, percpu bool) ([]float64, error) {
	// On non windows system invoke library funtions
	return cpuinfo.Percent(interval, percpu) //nolint:wrapcheck // Temporary Lib replacement
}
