package snclient

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/humanize"
	"github.com/consol-monitoring/snclient/pkg/utils"
	"github.com/shirou/gopsutil/v4/process"
)

func init() {
	RegisterModule(
		&AvailableTasks,
		"ProcessMemoryWatcher",
		"/settings/process memory",
		NewProcessMemoryWatcherHandler,
		ConfigInit{
			ConfigData{
				"check interval": "5s",
				"memory limit":   "512MiB",
			},
		},
	)
}

type ProcessMemoryWatcherHandler struct {
	noCopy noCopy

	snc *Agent

	stopChannel   chan bool
	memoryLimit   uint64
	checkInterval time.Duration
	running       atomic.Bool
}

func NewProcessMemoryWatcherHandler() Module {
	return &ProcessMemoryWatcherHandler{}
}

func (p *ProcessMemoryWatcherHandler) Init(snc *Agent, section *ConfigSection, _ *Config, _ *AgentRunSet) error {
	p.snc = snc
	p.stopChannel = make(chan bool)

	memoryLimit, _, err := section.GetBytes("memory limit")
	if err != nil {
		return fmt.Errorf("memory limit: %s", err.Error())
	}
	p.memoryLimit = memoryLimit

	checkInterval, _, err := section.GetDuration("check interval")
	if err != nil {
		return fmt.Errorf("check interval: %s", err.Error())
	}
	if checkInterval <= 0 {
		return fmt.Errorf("check interval: must be greater than 0")
	}
	p.checkInterval = time.Duration(checkInterval * float64(time.Second))

	return nil
}

func (p *ProcessMemoryWatcherHandler) Start() error {
	if p.memoryLimit == 0 {
		log.Tracef("process memory watcher disabled (memory limit: 0)")

		return nil
	}

	p.running.Store(true)
	go func() {
		defer p.snc.logPanicExit()
		p.mainLoop()
	}()

	return nil
}

func (p *ProcessMemoryWatcherHandler) Stop() {
	if !p.running.Load() {
		return
	}
	p.running.Store(false)
	close(p.stopChannel)
}

func (p *ProcessMemoryWatcherHandler) mainLoop() {
	log.Debugf(
		"starting process memory watcher (limit: %s, interval: %s)",
		humanize.BytesF(p.memoryLimit, 2),
		p.checkInterval.String(),
	)

	ticker := time.NewTicker(p.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChannel:
			log.Tracef("stopping process memory watcher")

			return
		case <-ticker.C:
			rss, err := p.currentRSS()
			if err != nil {
				log.Debugf("process memory watcher failed to get rss: %s", err.Error())

				continue
			}
			if rss <= p.memoryLimit {
				continue
			}

			log.Errorf(
				"process memory limit exceeded (rss: %s, limit: %s)",
				humanize.BytesF(rss, 2),
				humanize.BytesF(p.memoryLimit, 2),
			)
			utils.LogThreadDump(log)
			panic(fmt.Sprintf("process memory limit exceeded: rss=%d limit=%d", rss, p.memoryLimit))
		}
	}
}

func (p *ProcessMemoryWatcherHandler) currentRSS() (uint64, error) {
	pid32, err := convert.Int32E(os.Getpid())
	if err != nil {
		return 0, fmt.Errorf("pid conversion failed: %s", err.Error())
	}

	proc, err := process.NewProcess(pid32)
	if err != nil {
		return 0, fmt.Errorf("process lookup failed: %s", err.Error())
	}

	memInfo, err := proc.MemoryInfo()
	if err != nil {
		return 0, fmt.Errorf("memory info failed: %s", err.Error())
	}

	return memInfo.RSS, nil
}
