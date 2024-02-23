package snclient

import (
	"context"
	"fmt"
	"strings"
	"time"

	"pkg/eventlog"
	"pkg/utils"
)

func (l *CheckEventlog) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	timeZone, err := time.LoadLocation(l.timeZoneStr)
	if err != nil {
		return nil, fmt.Errorf("couldn't find timezone: %s", l.timeZoneStr)
	}

	if len(l.files) == 0 {
		filenames, err2 := eventlog.GetFileNames()
		if err2 != nil {
			return nil, fmt.Errorf("wmi query failed: %s", err2.Error())
		}
		l.files = append(l.files, filenames...)
	}

	lookBack, err := utils.ExpandDuration(l.scanRange)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse scan-range: %s", err.Error())
	}
	if lookBack < 0 {
		lookBack *= -1
	}
	scanLookBack := time.Now().Add(-time.Second * time.Duration(lookBack))

	for _, file := range l.files {
		log.Tracef("fetching eventlog: %s", file)
		fileEvent, err := eventlog.GetLog(file, scanLookBack)
		if err != nil {
			log.Warnf("eventlog query failed, file: %s: %s", file, err.Error())

			continue
		}

		for i := range fileEvent {
			event := fileEvent[i]
			timeWritten, _ := time.Parse(eventlog.WMIDateFormat, event.TimeWritten)
			message := event.Message
			if l.truncateMessage > 0 && len(event.Message) > l.truncateMessage {
				message = event.Message[:l.truncateMessage]
			}
			check.listData = append(check.listData, map[string]string{
				"computer":  event.ComputerName,
				"file":      event.LogFile,
				"log":       event.LogFile,
				"id":        fmt.Sprintf("%d", event.EventCode),
				"level":     strings.ToLower(event.Type),
				"message":   message,
				"provider":  event.SourceName,
				"source":    event.SourceName,
				"written":   timeWritten.In(timeZone).Format("2006-01-02 15:04:05 MST"),
				"writtenTS": fmt.Sprintf("%d", timeWritten.Unix()),
			})
		}
	}

	return check.Finalize()
}
