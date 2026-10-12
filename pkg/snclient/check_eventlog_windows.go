package snclient

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/eventlog"
	"github.com/consol-monitoring/snclient/pkg/utils"
)

//nolint:funlen // is not too long now to require splitting
func (l *CheckEventlog) Check(ctx context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	queryWhereLines, err := l.buildQueryWhereLines()
	if err != nil {
		return nil, fmt.Errorf("error when building query WHERE filter: %w", err)
	}

	if filterWhereLine := l.buildFilterQueryWhereLine(check.filter); filterWhereLine != "" {
		log.Tracef("pushing filter conditions into the wmi query: %s", filterWhereLine)
		queryWhereLines = append(queryWhereLines, filterWhereLine)
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
	uniqueIndexList := map[string]map[string]string{}
	filterUnique := false

	switch l.uniqueIndex {
	case "", "0", "false", "no":
		l.uniqueIndex = ""
	case "1":
		filterUnique = true
		l.uniqueIndex = DefaultUniqueIndex
	default:
		filterUnique = true
	}

	for _, file := range l.files {
		log.Tracef("fetching eventlog: %s", file)
		ctxWithLogger := utils.ContextWithLogger(ctx, log)
		fileEvent, err := l.queryLog(ctxWithLogger, file, scanLookBack, queryWhereLines)
		if err != nil {
			return nil, err
		}

		for i := range fileEvent {
			event := fileEvent[i]
			timeWritten, _ := l.ParseWMIDateTime(event.TimeWritten)
			message := event.Message
			if l.truncateMessage > 0 && len(event.Message) > l.truncateMessage {
				message = event.Message[:l.truncateMessage]
			}
			listData := map[string]string{
				"computer": event.ComputerName,
				"file":     event.LogFile,
				"log":      event.LogFile,
				"id":       fmt.Sprintf("%d", event.EventCode),
				"level":    strings.ToLower(event.Type),
				"message":  message,
				"provider": event.SourceName,
				"source":   event.SourceName,
				"written":  fmt.Sprintf("%d", timeWritten.Unix()),
			}
			if !filterUnique {
				check.listData = append(check.listData, listData)

				continue
			}

			// filter out duplicate events based on the unique-index argument
			uniqueID := ReplaceMacros(l.uniqueIndex, check.timezone, listData)
			log.Tracef("expanded unique filter: %s", uniqueID)
			if prevEntry, ok := uniqueIndexList[uniqueID]; ok {
				count := convert.Int64(prevEntry["_count"])
				prevEntry["_count"] = fmt.Sprintf("%d", count+1)
			} else {
				check.listData = append(check.listData, listData)
				listData["_count"] = "1"
				uniqueIndexList[uniqueID] = listData
			}
		}
	}

	return check.Finalize()
}

type eventlogQueryResult struct {
	events []eventlog.Event
	err    error
}

// queryLog runs the WMI query on a separate goroutine and bounds the wait by the check context.
// The goroutine may keep running in the background after a timeout, which is acceptable because it uses a dedicated WMI connection.
func (l *CheckEventlog) queryLog(ctx context.Context, file string, scanLookBack time.Time, queryWhereLines []string) ([]eventlog.Event, error) {
	resultCh := make(chan eventlogQueryResult, 1)
	go func() {
		events, err := eventlog.GetLog(ctx, file, scanLookBack, queryWhereLines)
		resultCh <- eventlogQueryResult{events: events, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("eventlog query timed out, file: %s: %w", file, ctx.Err())
	case res := <-resultCh:
		if res.err != nil {
			return nil, fmt.Errorf("eventlog query failed, file: %s: %w", file, res.err)
		}

		return res.events, nil
	}
}

// ParseWMIDateTime parses a WMI datetime string into a time.Time object.
func (l *CheckEventlog) ParseWMIDateTime(wmiDateTime string) (time.Time, error) {
	// Check if the string has at least 22 characters to avoid slicing errors
	if len(wmiDateTime) < 22 {
		return time.Time{}, fmt.Errorf("invalid WMI datetime string, must be at least 22 characters long")
	}

	// Extract the date and time components
	yearStr := wmiDateTime[0:4]
	monthStr := wmiDateTime[4:6]
	dayStr := wmiDateTime[6:8]
	hourStr := wmiDateTime[8:10]
	minuteStr := wmiDateTime[10:12]
	secondStr := wmiDateTime[12:14]
	microsecStr := wmiDateTime[15:21] // Skipping the dot at position 14
	offsetSign := wmiDateTime[21:22]
	offsetStr := wmiDateTime[22:]

	year, err := strconv.Atoi(yearStr)
	if err2 := l.checkRange("year", year, err, -1, -1); err2 != nil {
		return time.Time{}, err2
	}

	month, err := strconv.Atoi(monthStr)
	if err2 := l.checkRange("month", month, err, 1, 12); err2 != nil {
		return time.Time{}, err2
	}

	day, err := strconv.Atoi(dayStr)
	if err2 := l.checkRange("day", day, err, 1, 31); err2 != nil {
		return time.Time{}, err2
	}

	hour, err := strconv.Atoi(hourStr)
	if err2 := l.checkRange("hour", hour, err, 0, 23); err2 != nil {
		return time.Time{}, err2
	}

	minute, err := strconv.Atoi(minuteStr)
	if err2 := l.checkRange("minute", minute, err, 0, 59); err2 != nil {
		return time.Time{}, err2
	}

	second, err := strconv.Atoi(secondStr)
	if err2 := l.checkRange("second", second, err, 0, 59); err2 != nil {
		return time.Time{}, err2
	}

	microsec, err := strconv.Atoi(microsecStr)
	if err2 := l.checkRange("microsecond", microsec, err, -1, -1); err2 != nil {
		return time.Time{}, err2
	}

	offsetMinutes, err := strconv.Atoi(offsetStr)
	if err2 := l.checkRange("offset", offsetMinutes, err, -1, -1); err2 != nil {
		return time.Time{}, err2
	}

	// Apply the sign to the offset
	if offsetSign == "-" {
		offsetMinutes = -offsetMinutes
	} else if offsetSign != "+" {
		// Invalid sign, return current time
		return time.Time{}, fmt.Errorf("invalid offset sign, must be + or -")
	}

	// Convert offset from minutes to seconds
	offsetSeconds := offsetMinutes * 60

	// Create a fixed time zone based on the offset
	loc := time.FixedZone("WMI", offsetSeconds)

	// Construct the time.Time object
	t := time.Date(year, time.Month(month), day, hour, minute, second, microsec*1000, loc)

	return t, nil
}

func (l *CheckEventlog) checkRange(name string, value int, err error, minVal, maxVal int) error {
	if err != nil {
		return fmt.Errorf("invalid %s: %s", name, err.Error())
	}

	if minVal != -1 && value < minVal {
		return fmt.Errorf("%s out of range", name)
	}

	if maxVal != -1 && value > maxVal {
		return fmt.Errorf("%s out of range", name)
	}

	return nil
}

// Win32_NTLogEvent.EventType values, the numbers are consecutive, see the Win32_NTLogEvent class documentation.
const (
	CheckEventlogEventTypeError = iota + 1
	CheckEventlogEventTypeWarning
	CheckEventlogEventTypeInformation
	CheckEventlogEventTypeSuccessAudit
	CheckEventlogEventTypeFailureAudit
)

// EventCode is a uint16 property of Win32_NTLogEvent.
const CheckEventlogMaxEventCode = 65535

// CheckEventlogEventTypeNameMap keys must match normalizeEventTypeName() output.
var CheckEventlogEventTypeNameMap = map[string]uint64{
	"error":        CheckEventlogEventTypeError,
	"warning":      CheckEventlogEventTypeWarning,
	"information":  CheckEventlogEventTypeInformation,
	"successaudit": CheckEventlogEventTypeSuccessAudit,
	"auditsuccess": CheckEventlogEventTypeSuccessAudit,
	"failureaudit": CheckEventlogEventTypeFailureAudit,
	"auditfailure": CheckEventlogEventTypeFailureAudit,
}

func (l *CheckEventlog) buildQueryWhereLines() ([]string, error) {
	queryWhereLines := []string{}

	for _, filter := range l.eventCodesFilters {
		whereLine, err := l.buildEventCodeCondition(filter)
		if err != nil {
			return []string{}, fmt.Errorf("event-codes: %w", err)
		}
		queryWhereLines = append(queryWhereLines, whereLine)
	}

	for _, filter := range l.eventTypesFilters {
		whereLine, err := l.buildEventTypeCondition(filter)
		if err != nil {
			return []string{}, fmt.Errorf("event-types: %w", err)
		}
		queryWhereLines = append(queryWhereLines, whereLine)
	}

	for _, filter := range l.sourceNamesFilters {
		whereLine, err := l.buildSourceNameCondition(filter)
		if err != nil {
			return []string{}, fmt.Errorf("source-names: %w", err)
		}
		queryWhereLines = append(queryWhereLines, whereLine)
	}

	return queryWhereLines, nil
}

func (l *CheckEventlog) buildEventCodeCondition(filter string) (string, error) {
	values := l.splitFilterValues(filter)
	if len(values) == 0 {
		return "", fmt.Errorf("no event codes specified")
	}

	conditions := make([]string, 0, len(values))
	for _, value := range values {
		eventCode, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return "", fmt.Errorf("invalid event code %q: must be an integer between 0 and %d", value, CheckEventlogMaxEventCode)
		}
		conditions = append(conditions, fmt.Sprintf("EventCode = %d", eventCode))
	}

	return l.joinConditions(conditions, GroupOr), nil
}

func (l *CheckEventlog) buildEventTypeCondition(filter string) (string, error) {
	values := l.splitFilterValues(filter)
	if len(values) == 0 {
		return "", fmt.Errorf("no event types specified")
	}

	conditions := make([]string, 0, len(values))
	for _, value := range values {
		eventType, err := l.parseEventType(value)
		if err != nil {
			return "", err
		}
		conditions = append(conditions, fmt.Sprintf("EventType = %d", eventType))
	}

	return l.joinConditions(conditions, GroupOr), nil
}

func (l *CheckEventlog) buildSourceNameCondition(filter string) (string, error) {
	values := l.splitFilterValues(filter)
	if len(values) == 0 {
		return "", fmt.Errorf("no source names specified")
	}

	conditions := make([]string, 0, len(values))
	for _, value := range values {
		if err := l.checkWQLString(value); err != nil {
			return "", err
		}
		conditions = append(conditions, fmt.Sprintf("SourceName LIKE '%s'", value))
	}

	return l.joinConditions(conditions, GroupOr), nil
}

func (l *CheckEventlog) parseEventType(value string) (uint64, error) {
	if eventType, err := strconv.ParseUint(value, 10, 8); err == nil {
		if eventType >= CheckEventlogEventTypeError && eventType <= CheckEventlogEventTypeFailureAudit {
			return eventType, nil
		}

		return 0, fmt.Errorf("invalid event type %d: valid event types are: error, warning, information, success audit, failure audit, or the numbers 1 to 5", eventType)
	}

	if eventType, ok := CheckEventlogEventTypeNameMap[l.normalizeEventTypeName(value)]; ok {
		return eventType, nil
	}

	return 0, fmt.Errorf("invalid event type %q: valid event types are: error, warning, information, success audit, failure audit, or the numbers 1 to 5", value)
}

func (l *CheckEventlog) normalizeEventTypeName(name string) string {
	eventNameReplacer := strings.NewReplacer(" ", "", "_", "", "-", "")

	return eventNameReplacer.Replace(strings.ToLower(name))
}

func (l *CheckEventlog) checkWQLString(value string) error {
	// single quotes would end the WQL string literal and allow query injection
	wqlBlacklistRunes := []rune{'\'', '\\'}

	if strings.ContainsFunc(value, func(r rune) bool { return slices.Contains(wqlBlacklistRunes, r) }) {
		return fmt.Errorf("source name %q contains one of the blacklisted characters: %q", value, string(wqlBlacklistRunes))
	}
	if strings.ContainsFunc(value, func(r rune) bool { return !unicode.IsPrint(r) }) {
		return fmt.Errorf("source name %q contains non printable characters", value)
	}

	return nil
}

func (l *CheckEventlog) splitFilterValues(filter string) []string {
	values := []string{}
	for value := range strings.SplitSeq(filter, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		values = append(values, value)
	}

	return values
}

// joinConditions combines conditions with the given logical operator.
func (l *CheckEventlog) joinConditions(conditions []string, operator GroupOperator) string {
	switch len(conditions) {
	case 0:
		return ""
	case 1:
		return conditions[0]
	}

	logicalOperator := " AND "
	if operator == GroupOr {
		logicalOperator = " OR "
	}

	return fmt.Sprintf("(%s)", strings.Join(conditions, logicalOperator))
}
