package snclient

func init() {
	AvailableChecks["check_eventlog"] = CheckEntry{"check_eventlog", NewCheckEventlog}
}

type CheckEventlog struct {
	files           []string
	timeZoneStr     string
	scanRange       string
	truncateMessage int
	uniqueIndex     string
}

func NewCheckEventlog() CheckHandler {
	return &CheckEventlog{
		timeZoneStr: "Local",
		scanRange:   "-24h",
	}
}

func (l *CheckEventlog) Build() *CheckData {
	return &CheckData{
		name: "check_eventlog",
		description: `Checks the windows eventlog entries.

Basically, this check wraps this wmi query:
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

See https://learn.microsoft.com/en-us/previous-versions/windows/desktop/eventlogprov/win32-ntlogevent for
a description of the provided fields.
`,
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
			"unique-index":     {value: &l.uniqueIndex, description: "Combination of fields that identify unique events (a good choice is \"${log}-${source}-${id}\")"},
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
			{name: "written", description: "Time of the message being written( TimeWritten)"},
		},
		exampleDefault: `
    check_eventlog
    OK - Event log seems fine

Only return unique events:

	check_eventlog "detail-syntax=%(id) %(uniqueindex)" "unique-index=1" 
	WARNING - 4 message(s) warning(10010 Application-Microsoft-Windows-RestartManager-10010, 10016 System-Microsoft-Windows-DistributedCOM-10016, 6155 System-LsaSrv-6155, 6147 System-LsaSrv-6147)
	`,
		exampleArgs: `filter=provider = 'Microsoft-Windows-Security-SPP' and id = 903 and message like 'foo'`,
	}
}
