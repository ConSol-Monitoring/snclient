package snclient

import (
	"context"
	"fmt"
	"strings"
	"time"

	"pkg/eventlog"
	"pkg/utils"
	"pkg/wmi"
)

func init() {
	AvailableChecks["check_eventlog"] = CheckEntry{"check_eventlog", NewCheckEventlog}
}

type CheckEventlog struct {
	files           []string
	timeZoneStr     string
	scanRange       string
	truncateMessage int
}

func NewCheckEventlog() CheckHandler {
	return &CheckEventlog{
		timeZoneStr: "Local",
		scanRange:   "-24h",
	}
}

func (l *CheckEventlog) Build() *CheckData {
	return &CheckData{
		name:        "check_eventlog",
		description: "Checks the windows eventlog entries.",
		implemented: Windows,
		result: &CheckResult{
			State: CheckExitOK,
		},
		hasInventory: NoCallInventory,
		args: map[string]CheckArgument{
			"file":             {value: &l.files, description: "File to read (can be specified multiple times to check multiple files)"},
			"log":              {value: &l.files, description: "Alias for file"},
			"timezone":         {value: &l.timeZoneStr, description: "Sets the timezone for time metrics (default is local time)"},
			"scan-range":       {value: &l.scanRange, description: "Sets time range to scan for message (default is 24h)"},
			"truncate-message": {value: &l.truncateMessage, description: "Maximum length of message for each event log message text"},
		},
		defaultFilter:   "level in ('warning', 'error', 'critical')",
		defaultWarning:  "level = 'warning' or problem_count > 0",
		defaultCritical: "level in ('error', 'critical')",
		detailSyntax:    "%(file) %(source) (%(message))",
		okSyntax:        "%(status): Event log seems fine",
		topSyntax:       "%(status): %(count) message(s) %(problem_list)",
		emptySyntax:     "%(status): No entries found",
		emptyState:      0,
		attributes: []CheckAttribute{
			{name: "computer", description: "Which computer generated the message"},
			{name: "file", description: "The logfile name"},
			{name: "log", description: "Alias for file"},
			{name: "id", description: "Eventlog id"},
			{name: "keyword", description: "Keyword(s) associated with the event"},
			{name: "level", description: "Severity level (lowercase)"},
			{name: "message", description: "The message as a string"},
			{name: "source", description: "The source system"},
			{name: "provider", description: "Alias for source"},
			{name: "task", description: "The type of event"},
			{name: "written", description: "Time of the message being written"},
		},
		exampleDefault: `
    check_eventlog
    OK: Event log seems fine
	`,
		exampleArgs: `filter=provider = 'Microsoft-Windows-Security-SPP' and id = 903 and message like 'foo'`,
	}
}

func (l *CheckEventlog) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	timeZone, err := time.LoadLocation(l.timeZoneStr)
	if err != nil {
		return nil, fmt.Errorf("couldn't find timezone: %s", l.timeZoneStr)
	}

	if len(l.files) == 0 {
		query := "SELECT LogfileName FROM Win32_NTEventLogFile"
		res, err := wmi.QueryDefaultRetry(query)
		if err != nil {
			return nil, fmt.Errorf("wmi query failed: %s", err.Error())
		}
		for _, row := range res {
			l.files = append(l.files, row["LogfileName"])
		}
	}

	lookBack, err := utils.ExpandDuration(l.scanRange)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse scan-range: %s", err.Error())
	}
	if lookBack < 0 {
		lookBack *= -1
	}
	scanLookBack := uint64(lookBack)

	for _, file := range l.files {
		e := eventlog.NewEventLog(file, log)
		e.TimeDiff = scanLookBack
		fileEvent, err := e.Query()
		if err != nil {
			log.Warnf("eventlog query failed, file: %s: %s", file, err.Error())

			continue
		}

		for _, event := range fileEvent {
			message := event.Message
			if l.truncateMessage > 0 && len(event.Message) > l.truncateMessage {
				message = event.Message[:l.truncateMessage]
			}

			check.listData = append(check.listData, map[string]string{
				"computer": event.Computer,
				"file":     event.Channel,
				"log":      event.Channel,
				"id":       fmt.Sprintf("%d", event.EventIdentifier.ID),
				"keyword":  strings.Join(event.Keywords, ","),
				"level":    strings.ToLower(event.Level),
				"message":  message,
				"provider": event.Provider.Name,
				"source":   event.Provider.Name,
				"task":     event.Task,
				"written":  event.TimeCreated.SystemTime.In(timeZone).Format("2006-01-02 15:04:05 MST"),
			})
		}
	}

	return check.Finalize()
}
