package snclient

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"pkg/humanize"
)

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

func (l *LogrotateHandler) Defaults() ConfigData {
	defaults := ConfigData{
		"max size": "0",
	}

	return defaults
}

func (l *LogrotateHandler) Init(snc *Agent, section *ConfigSection, _ *Config, _ *ModuleSet) error {
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
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopChannel:
			log.Tracef("stopping LogrotateHandler mainLoop")

			return
		case <-ticker.C:
			// skip rotation if we don't log into a file atm
			if targetWriter == os.Stdout || targetWriter != os.Stderr {
				continue
			}
			logFile, _ := l.snc.Config.Section("/settings/log").GetString("file name")
			fileInfo, err := os.Stat(logFile)
			if err != nil {
				log.Tracef("[Logrotate] stat failed: %s: %s", logFile, err.Error())

				continue
			}

			log.Tracef("[Logrotate] check logfile rotation (threshold %s / current size: %s)",
				humanize.IBytes(l.maxSize),
				humanize.IBytes(uint64(fileInfo.Size())),
			)
			if uint64(fileInfo.Size()) > l.maxSize {
				l.rotate(logFile, l.maxSize/2)
			}

			continue
		}
	}
}

func (l *LogrotateHandler) rotate(logFile string, targetsize uint64) {
	log.Infof("[Logrotate] rotating logfile to %s", humanize.IBytes(targetsize))

	lineCount, err := l.numLines2Remove(logFile, targetsize)
	if err != nil {
		log.Errorf("counting lines %s failed: %s", logFile, err.Error())

		return
	}
	log.Tracef("[Logrotate] removing %d lines", lineCount)

	newFile, err := os.OpenFile(logFile+".tmp", os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		log.Errorf(fmt.Sprintf("failed to open logfile %s.tmp: %s", logFile, err.Error()))

		return
	}
	defer os.Remove(logFile + ".tmp")

	file, err := os.Open(logFile)
	if err != nil {
		log.Errorf("open %s failed: %s", logFile, err.Error())
		newFile.Close()

		return
	}

	skipped := int64(0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		skipped++
		if skipped >= lineCount {
			_, err = newFile.Write(scanner.Bytes())
			if err != nil {
				log.Errorf(fmt.Sprintf("failed to write tmp logfile %s.tmp: %s", logFile, err.Error()))
				newFile.Close()

				return
			}
			_, err = newFile.Write([]byte{'\n'})
			if err != nil {
				log.Errorf(fmt.Sprintf("failed to write tmp logfile %s.tmp: %s", logFile, err.Error()))
				newFile.Close()

				return
			}
		}
	}
	err = newFile.Close()
	if err != nil {
		log.Errorf(fmt.Sprintf("failed to write tmp logfile %s.tmp: %s", logFile, err.Error()))

		return
	}

	err = os.Rename(logFile+".tmp", logFile)
	if err != nil {
		log.Errorf(fmt.Sprintf("failed to rename tmp logfile %s.tmp: %s", logFile, err.Error()))

		return
	}
}

func (l *LogrotateHandler) numLines2Remove(logFile string, targetsize uint64) (lineCount int64, err error) {
	fileInfo, err := os.Stat(logFile)
	if err != nil {
		return 0, fmt.Errorf("stat %s failed: %s", logFile, err.Error())
	}

	curSize := uint64(fileInfo.Size())
	removeSize := curSize - targetsize

	file, err := os.Open(logFile)
	if err != nil {
		return 0, fmt.Errorf("open %s failed: %s", logFile, err.Error())
	}

	defer file.Close()

	sizeCount := uint64(0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		sizeCount += uint64(len(scanner.Bytes()))
		lineCount++
		if sizeCount > removeSize {
			break
		}
	}

	return lineCount, nil
}
