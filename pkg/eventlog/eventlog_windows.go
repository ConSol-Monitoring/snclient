package eventlog

import (
	"fmt"
	"time"

	"github.com/consol-monitoring/snclient/pkg/wmi"
)

const (
	WMIDateFormat = "20060102150405.000000"
)

type EventLog struct {
	LogfileName string
}

func GetFileNames() ([]string, error) {
	filenames := []EventLog{}
	query := "SELECT LogfileName FROM Win32_NTEventLogFile"
	err := wmi.QueryDefaultRetry(query, &filenames)
	if err != nil {
		return nil, fmt.Errorf("wmi query failed: %s", err.Error())
	}

	files := make([]string, 0, len(filenames))
	for _, row := range filenames {
		files = append(files, row.LogfileName)
	}

	return files, nil
}

type Event struct {
	ComputerName    string
	LogFile         string
	Message         string
	SourceName      string
	Type            string
	TimeWritten     string
	TimeGenerated   string
	Category        uint64
	EventCode       uint64
	EventIdentifier uint64
	EventType       uint64
}

func GetLog(file string, newerThan time.Time) ([]Event, error) {
	messages := []Event{}

	// Format the time without the timezone offset
	formattedTime := newerThan.Format(WMIDateFormat)

	// Get the timezone offset in seconds and convert it to minutes
	_, offsetSeconds := newerThan.Zone()
	offsetMinutes := offsetSeconds / 60

	// Determine the sign of the offset
	offsetSign := "+"
	if offsetMinutes < 0 {
		offsetSign = "-"
		offsetMinutes = -offsetMinutes // Make offsetMinutes positive for formatting
	}

	// append the offset with three digits and leading zeros
	wmiFormattedTime := fmt.Sprintf("%s%s%03d", formattedTime, offsetSign, offsetMinutes)

	query := fmt.Sprintf(`
	SELECT
		ComputerName,
		LogFile,
		Category,
		EventCode,
		EventIdentifier,
		EventType,
		Message,
		SourceName,
		Type,
		TimeWritten,
		TimeGenerated
	FROM
		Win32_NTLogEvent
	WHERE
		Logfile='%s'
		AND TimeGenerated >= '%s'
	`, file, wmiFormattedTime)
	err := wmi.QueryDefaultRetry(query, &messages)
	if err != nil {
		return nil, fmt.Errorf("wmi query failed: %s", err.Error())
	}

	return messages, nil
}
