package snclient

func init() {
	AvailableChecks["check_eventlog"] = CheckEntry{"check_eventlog", NewCheckEventlog}
}

const DefaultUniqueIndex = "%(log)-%(source)-%(id)"

type CheckEventlog struct {
	files             []string
	scanRange         string
	truncateMessage   int
	uniqueIndex       string
	eventCodesFilters []string
}

func NewCheckEventlog() CheckHandler {
	return &CheckEventlog{
		scanRange:   "-24h",
		uniqueIndex: "1",
	}
}

func (l *CheckEventlog) Build() *CheckData {
	return &CheckData{
		name: "check_eventlog",
		description: `Checks Windows event log entries.

This check wraps the following WMI query:

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

See the [field descriptions](https://learn.microsoft.com/en-us/previous-versions/windows/desktop/eventlogprov/win32-ntlogevent).

When possible, SNClient adds filter conditions on id, level and source (or provider) to the WMI query's WHERE clause.
SNClient still applies the filter to the returned events.`,
		implemented: Windows,
		result: &CheckResult{
			State: CheckExitOK,
		},
		hasInventory: NoCallInventory,
		args: map[string]CheckArgument{
			"file":             {value: &l.files, description: "File to read. Repeat this option to check multiple files."},
			"log":              {value: &l.files, description: "Alias for file"},
			"timezone":         {description: "Set the timezone for time metrics (default is local time)"},
			"scan-range":       {value: &l.scanRange, description: "Time range to search for messages (default is 24h)"},
			"truncate-message": {value: &l.truncateMessage, description: "Maximum length of each event log message"},
			"unique-index":     {value: &l.uniqueIndex, description: "Fields that identify unique events. Set to 0 to disable. Default is \"" + DefaultUniqueIndex + "\""},
			"unique":           {value: &l.uniqueIndex, description: "Alias for unique-index"},
			"event-codes": {value: &l.eventCodesFilters, isFilter: true, description: "Event codes to query. SNClient combines comma-separated values with OR. " +
				"Valid range: 0-65535. You can repeat this option."},
		},
		defaultFilter:   "level in ('warning', 'error', 'critical')",
		defaultWarning:  "level = 'warning' or problem_count > 0",
		defaultCritical: "level in ('error', 'critical')",
		detailSyntax:    "%(file) %(source) (%(message))",
		okSyntax:        "%(status) - Event log seems fine",
		topSyntax:       "%(status) - %(count) message(s) %(problem_list)",
		emptySyntax:     "%(status) - No entries found",
		emptyState:      0,
		attributes: []CheckAttribute{
			{name: "computer", description: "Which computer generated the message (ComputerName)"},
			{name: "file", description: "The logfile name"},
			{name: "log", description: "Alias for file"},
			{name: "id", description: "Eventlog id (EventCode)"},
			{name: "eventidentifier", description: "Event identifier (EventIdentifier)"},
			{name: "level", description: "Severity level (lowercase Type)"},
			{name: "message", description: "The message as a string"},
			{name: "source", description: "The source system (SourceName)"},
			{name: "provider", description: "Alias for source"},
			{name: "written", description: "Time of the message being written (TimeWritten)", unit: UDate},
		},
		exampleDefault: `
    check_eventlog
    OK - Event log seems fine

    Return all events. By default, the check returns only unique events:

    check_eventlog "detail-syntax=%(id) %(uniqueindex)" "unique-index=0"
    WARNING - 4 message(s) warning(10010 Application-Microsoft-Windows-RestartManager-10010, 10016 System-Microsoft-Windows-DistributedCOM-10016, 6155 System-LsaSrv-6155, 6147 System-LsaSrv-6147)
	`,
		exampleArgs: `filter=provider = 'Microsoft-Windows-Security-SPP' and id = 903 and message like 'foo'`,
	}
}
