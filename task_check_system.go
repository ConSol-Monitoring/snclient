package snclient

import (
	"fmt"
	"time"

	cpuinfo "github.com/shirou/gopsutil/v3/cpu"
)

type CheckSystemHandler struct {
	noCopy noCopy

	stopChannel chan bool
	snc         *Agent

	bufferLength float64
}

func NewCheckSCheckSystemHandler() TaskHandler {
	var h TaskHandler = &CheckSystemHandler{}

	return h
}

func (c *CheckSystemHandler) Defaults() ConfigData {
	defaults := ConfigData{
		"default buffer length": "1h",
	}

	return defaults
}

func (c *CheckSystemHandler) Init(snc *Agent, conf *ConfigSection) error {
	c.snc = snc
	c.stopChannel = make(chan bool)

	bufferLength, _, err := conf.GetDuration("default buffer length")
	if err != nil {
		return fmt.Errorf("default buffer length: %s", err.Error())
	}
	c.bufferLength = bufferLength

	return nil
}

func (c *CheckSystemHandler) Start() error {
	// create counter
	data, err := c.fetch()
	if err != nil {
		return fmt.Errorf("cpuinfo failed: %s", err.Error())
	}
	for key := range data {
		c.snc.Counter.Create("cpu", key, c.bufferLength)
	}

	go c.mainLoop()

	return nil
}

func (c *CheckSystemHandler) Stop() {
	close(c.stopChannel)
}

func (c *CheckSystemHandler) mainLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChannel:
			log.Tracef("stopping CheckSystemUnix mainLoop")

			return
		case <-ticker.C:
			data, err := c.fetch()
			if err != nil {
				log.Warnf("[CheckSystemUnix] reading cpu info failed: %s", err.Error())

				continue
			}

			for key, val := range data {
				c.snc.Counter.Set("cpu", key, val)
			}

			continue
		}
	}
}

func (c *CheckSystemHandler) fetch() (data map[string]float64, err error) {
	data = map[string]float64{}

	infoAll, err := cpuinfo.Percent(0, false)
	if err != nil {
		return nil, fmt.Errorf("cpuinfo failed: %s", err.Error())
	}
	data["total"] = infoAll[0]

	info, err := cpuinfo.Percent(0, true)
	if err != nil {
		return nil, fmt.Errorf("cpuinfo failed: %s", err.Error())
	}

	for i, d := range info {
		data[fmt.Sprintf("core%d", i)] = d
	}

	return data, nil
}
