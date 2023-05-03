package snclient

import (
	"strconv"
	"strings"

	"internal/eventlog"

	"github.com/elastic/beats/v7/winlogbeat/sys/winevent"
)

func init() {
	AvailableChecks["check_eventlog"] = CheckEntry{"check_eventlog", new(CheckEventlog)}
}

type CheckEventlog struct {
	noCopy noCopy
	data   CheckData
}

/* check_process_windows
 * Description: Checks the eventlog of the host.
 */

func (l *CheckEventlog) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(CheckExitOK)
	l.data.detailSyntax = "%(file) %(source) (%(message))"
	l.data.okSyntax = "Event log seems fine"
	l.data.topSyntax = "%(count) message(s) %(problem_list)"
	l.data.emptySyntax = "No entries found"
	argList, _ := ParseArgs(args, &l.data)
	var output string
	files := []string{}
	var checkData map[string]string
	//var scanRange string
	events := []*winevent.Event{}

	// parse args
	for _, arg := range argList {
		switch arg.key {
		case "file", "log":
			files = append(files, arg.value)
			/*case "scan-range":
			scanRange = arg.value*/
		}
	}

	for _, file := range files {

		e := eventlog.EventLog{LogChannel: file}
		fileEvent, _ := e.Query()
		events = append(events, fileEvent...)

	}

	okList := make([]string, 0, len(events))
	warnList := make([]string, 0, len(events))
	critList := make([]string, 0, len(events))

	for _, event := range events {

		metrics := map[string]string{
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
		}

		switch {
		case CompareMetrics(metrics, l.data.critThreshold) && l.data.critThreshold.name != "none":
			critList = append(critList, ParseSyntax(l.data.detailSyntax, metrics))
		case CompareMetrics(metrics, l.data.warnThreshold) && l.data.warnThreshold.name != "none":
			warnList = append(warnList, ParseSyntax(l.data.detailSyntax, metrics))
		default:
			okList = append(okList, ParseSyntax(l.data.detailSyntax, metrics))
		}
	}

	if len(critList) > 0 {
		state = CheckExitCritical
	} else if len(warnList) > 0 {
		state = CheckExitWarning
	}

	checkData = map[string]string{
		"status":       strconv.FormatInt(state, 10),
		"count":        strconv.FormatInt(int64(len(events)), 10),
		"ok_list":      strings.Join(okList, ", "),
		"warn_list":    strings.Join(warnList, ", "),
		"crit_list":    strings.Join(critList, ", "),
		"problem_list": strings.Join(append(critList, warnList...), ", "),
	}

	if state == CheckExitOK {
		output = ParseSyntax(l.data.okSyntax, checkData)
	} else {
		output = ParseSyntax(l.data.topSyntax, checkData)
	}

	return &CheckResult{
		State:  state,
		Output: output,
	}, nil

}
