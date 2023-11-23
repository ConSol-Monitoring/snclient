package snclient

import (
	"context"
	"fmt"
	"pkg/eventlog"
	"pkg/wmi"
	"strconv"

	"github.com/elastic/beats/v7/winlogbeat/sys/winevent"
)

func init() {
	AvailableChecks["check_eventlog"] = CheckEntry{"check_eventlog", new(CheckEventlog)}
}

type CheckEventlog struct {
	files []string
}

func (l *CheckEventlog) Build() *CheckData {
	return &CheckData{
		name:        "check_eventlog",
		description: "Checks the windows eventlog entries.",
		implemented: Windows,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"file": {value: &l.files, description: "File to read (can be specified multiple times to check multiple files)"},
			"log":  {value: &l.files, description: "Alias for file"},
		},
		detailSyntax: "%(file) %(source) (%(message))",
		okSyntax:     "%(status): Event log seems fine",
		topSyntax:    "%(status): %(count) message(s) %(problem_list)",
		emptySyntax:  "No entries found",
		emptyState:   3,
	}
}

func (l *CheckEventlog) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	events := []*winevent.Event{}

	if len(l.files) == 0 {
		query := "SELECT LogfileName FROM Win32_NTEventLogFile"
		res, err := wmi.Query(query)
		if err != nil {
			return nil, fmt.Errorf("wmi query failed: %s", err.Error())
		}
		for _, row := range wmi.ResultToMap(res) {
			l.files = append(l.files, row["LogfileName"])
		}
	}

	for _, file := range l.files {
		e := eventlog.NewEventLog(file, log)
		fileEvent, _ := e.Query()
		events = append(events, fileEvent...)
	}

	for _, event := range events {
		check.listData = append(check.listData, map[string]string{
			"computer": event.Computer,
			"file":     event.Channel,
			"log":      event.Channel,
			"id":       strconv.FormatUint(event.RecordID, 10),
			"keyword":  event.Keywords[0],
			"level":    event.Level,
			"message":  event.Message,
			"provider": event.Provider.Name,
			"source":   event.Provider.Name,
			"task":     event.Task,
			"written":  event.TimeCreated.SystemTime.Format("02-01-2006 15:04:05"),
		})
	}

	return check.Finalize()
}
