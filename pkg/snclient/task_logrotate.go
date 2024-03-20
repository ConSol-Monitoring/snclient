package snclient

import (
	"fmt"
	"os"
	"time"

	"pkg/humanize"
)

const rotateCheckInterval = 60 * time.Second

func init() {
	RegisterModule(&AvailableTasks, "Logrotate", "/settings/log/file", NewLogrotateHandler)
}

type LogrotateHandler struct {
	noCopy noCopy

	stopChannel chan bool
	snc         *Agent

	maxSize uint64
}

func NewLogrotateHandler() Module {
	return &LogrotateHandler{}
}

func (l *LogrotateHandler) Defaults(_ *AgentRunSet) ConfigData {
	defaults := ConfigData{
		"max size": "0",
	}

	return defaults
}

func (l *LogrotateHandler) Init(snc *Agent, section *ConfigSection, _ *Config, _ *AgentRunSet) error {
	l.snc = snc
	l.stopChannel = make(chan bool)

	maxSize, _, err := section.GetBytes("max size")
	if err != nil {
		return fmt.Errorf("max size: %s", err.Error())
	}
	l.maxSize = maxSize

	return nil
}

func (l *LogrotateHandler) Start() error {
	go l.mainLoop()

	return nil
}

func (l *LogrotateHandler) Stop() {
	close(l.stopChannel)
}

func (l *LogrotateHandler) mainLoop() {
	if l.maxSize <= 0 {
		log.Tracef("automatic rotation is disabled")

		return
	}

	log.Tracef("starting LogrotateHandler mainLoop")
	ticker := time.NewTicker(rotateCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopChannel:
			log.Tracef("stopping LogrotateHandler mainLoop")

			return
		case <-ticker.C:
			// skip rotation if we don't log into a file atm
			if LogFileHandle == nil {
				log.Tracef("rotate skipped, file logger not active")

				continue
			}
			logFile, _ := l.snc.config.Section("/settings/log").GetString("file name")
			fileInfo, err := os.Stat(logFile)
			if err != nil {
				log.Tracef("stat failed: %s: %s", logFile, err.Error())

				continue
			}

			log.Tracef("check logfile rotation (threshold %s / current size: %s)",
				humanize.IBytes(l.maxSize),
				humanize.IBytes(uint64(fileInfo.Size())),
			)
			if uint64(fileInfo.Size()) > l.maxSize {
				l.rotate(logFile)
			}

			continue
		}
	}
}

func (l *LogrotateHandler) rotate(logFile string) {
	log.Debugf("rotating logfile %s", logFile)

	// remove previously rotated logfile
	os.Remove(logFile + ".old")

	if LogFileHandle != nil {
		err := LogFileHandle.Close()
		if err != nil {
			log.Errorf("failed to close log handle: %s", err.Error())

			return
		}
	}

	err := os.Rename(logFile, logFile+".old")
	if err != nil {
		log.Errorf("failed to rename logfile %s %s.old: %s", logFile, logFile, err.Error())

		return
	}

	// reopen logfile
	l.snc.createLogger(l.snc.config)

	log.Infof("rotated logfile to %s.old", logFile)
}
