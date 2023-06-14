package snclient

import (
	"strconv"

	"pkg/eventlog"

	"github.com/elastic/beats/v7/winlogbeat/sys/winevent"
)

func init() {
	AvailableChecks["check_eventlog"] = CheckEntry{"check_eventlog", new(CheckEventlog)}
}

type CheckEventlog struct{}

/* check_process_windows
 * Description: Checks the eventlog of the host.
 */

func (l *CheckEventlog) Check(_ *Agent, args []string) (*CheckResult, error) {
	check := &CheckData{
		result: &CheckResult{
			State: CheckExitOK,
		},
		detailSyntax: "%(file) %(source) (%(message))",
		okSyntax:     "%(status): Event log seems fine",
		topSyntax:    "%(status): %(count) message(s) %(problem_list)",
		emptySyntax:  "No entries found",
		emptyState:   3,
	}
	argList, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	files := []string{}
	events := []*winevent.Event{}

	// parse args
	for _, arg := range argList {
		switch arg.key {
		case "file", "log":
			files = append(files, arg.value)
		}
	}

	for _, file := range files {
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
