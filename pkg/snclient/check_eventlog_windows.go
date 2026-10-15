package snclient

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/eventlog"
	"github.com/consol-monitoring/snclient/pkg/utils"
	"github.com/consol-monitoring/snclient/pkg/wmi"
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
		filenames, err2 := getFileNames(ctx)
		if err2 != nil {
			return nil, fmt.Errorf("wmi query failed: %w", err2)
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

func getFileNames(ctx context.Context) ([]string, error) {
	type logfileNameResult struct {
		LogfileName string
	}

	type channelType struct {
		filenames []string
		err       error
	}

	resultCh := make(chan channelType, 1)
	query := "SELECT LogfileName FROM Win32_NTEventLogFile"

	go func() {
		queryResults := []logfileNameResult{}
		err := wmi.QueryIsolatedDefaultRetry(query, &queryResults)
		if err != nil {
			resultCh <- channelType{nil, err}

			return
		}

		filenames := make([]string, 0, len(queryResults))
		for _, queryResult := range queryResults {
			filenames = append(filenames, queryResult.LogfileName)
		}

		resultCh <- channelType{filenames, nil}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("get logfile names query: '%s' , timed out: %w", query, ctx.Err())
	case res := <-resultCh:
		if res.err != nil {
			return nil, fmt.Errorf("get logfile names query: '%s' , got an error: %w", query, res.err)
		}

		return res.filenames, nil
	}
}

// queryLog runs the WMI query on a separate goroutine and bounds the wait by the check context.
// The goroutine may keep running in the background after a timeout, which is acceptable because it uses a dedicated WMI connection.
func (l *CheckEventlog) queryLog(ctx context.Context, file string, scanLookBack time.Time, queryWhereLines []string) ([]eventlog.Event, error) {
	type channelType struct {
		events []eventlog.Event
		err    error
	}

	resultCh := make(chan channelType, 1)
	go func() {
		events, err := eventlog.GetLog(ctx, file, scanLookBack, queryWhereLines)
		resultCh <- channelType{events: events, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("eventlog query, file: '%s' , timed out: %w", file, ctx.Err())
	case res := <-resultCh:
		if res.err != nil {
			return nil, fmt.Errorf("eventlog query, file: '%s' , got an error: %w", file, res.err)
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

// EventCode is a uint16 property of Win32_NTLogEvent.
const CheckEventlogMaxEventCode = 65535

func (l *CheckEventlog) buildQueryWhereLines() ([]string, error) {
	queryWhereLines := []string{}

	for _, filter := range l.eventCodesFilters {
		whereLine, err := l.buildEventCodeCondition(filter)
		if err != nil {
			return []string{}, fmt.Errorf("event-codes: %w", err)
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

	return l.joinConditions(conditions), nil
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

// joinConditions combines multiple conditions with OR and wraps them in brackets.
func (l *CheckEventlog) joinConditions(conditions []string) string {
	switch len(conditions) {
	case 0:
		return ""
	case 1:
		return conditions[0]
	}

	return fmt.Sprintf("(%s)", strings.Join(conditions, " OR "))
}
