package snclient

import (
	"fmt"
	"time"

	cpuinfo "github.com/shirou/gopsutil/v3/cpu"
)

const (
	// CPUMeasureInterval sets the ticker measuring the CPU counter
	CPUMeasureInterval = 1 * time.Second
)

type CheckSystemHandler struct {
	noCopy noCopy

	stopChannel chan bool
	snc         *Agent

	bufferLength float64
}

func NewCheckSystemHandler() Module {
	return &CheckSystemHandler{}
}

func (c *CheckSystemHandler) Defaults() ConfigData {
	defaults := ConfigData{
		"default buffer length": "1h",
	}

	return defaults
}

func (c *CheckSystemHandler) Init(snc *Agent, section *ConfigSection, _ *Config, _ *ModuleSet) error {
	c.snc = snc
	c.stopChannel = make(chan bool)

	bufferLength, _, err := section.GetDuration("default buffer length")
	if err != nil {
		return fmt.Errorf("default buffer length: %s", err.Error())
	}
	c.bufferLength = bufferLength

	// create counter
	c.update(true)

	return nil
}

func (c *CheckSystemHandler) Start() error {
	go c.mainLoop()

	return nil
}

func (c *CheckSystemHandler) Stop() {
	close(c.stopChannel)
}

func (c *CheckSystemHandler) mainLoop() {
	ticker := time.NewTicker(CPUMeasureInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChannel:
			log.Tracef("stopping CheckSystem mainLoop")

			return
		case <-ticker.C:
			c.update(false)

			continue
		}
	}
}

func (c *CheckSystemHandler) update(create bool) {
	data, times, err := c.fetch()
	if err != nil {
		log.Warnf("[CheckSystem] reading cpu info failed: %s", err.Error())

		return
	}

	if create {
		for key := range data {
			c.snc.Counter.Create("cpu", key, c.bufferLength)
		}
		c.snc.Counter.CreateAny("cpuinfo", "info", c.bufferLength)
	}

	for key, val := range data {
		c.snc.Counter.Set("cpu", key, val)
	}
	c.snc.Counter.SetAny("cpuinfo", "info", times)
}

func (c *CheckSystemHandler) fetch() (data map[string]float64, cputimes *cpuinfo.TimesStat, err error) {
	data = map[string]float64{}

	infoAll, err := cpuinfo.Percent(0, false)
	if err != nil {
		return nil, nil, fmt.Errorf("cpuinfo failed: %s", err.Error())
	}
	data["total"] = infoAll[0]

	info, err := cpuinfo.Percent(0, true)
	if err != nil {
		return nil, nil, fmt.Errorf("cpuinfo failed: %s", err.Error())
	}

	for i, d := range info {
		data[fmt.Sprintf("core%d", i)] = d
	}

	times, err := cpuinfo.Times(false)
	if err != nil {
		return nil, nil, fmt.Errorf("cpuinfo failed: %s", err.Error())
	}

	return data, &times[0], nil
}
