package snclient

type CheckEntry struct {
	Name    string
	Handler CheckHandler
}

var AvailableChecks = make(map[string]CheckEntry)

const (
	// CheckExitOK is used for normal exits.
	CheckExitOK = 0

	// CheckExitWarning is used for warnings.
	CheckExitWarning = 1

	// CheckExitCritical is used for critical errors.
	CheckExitCritical = 2

	// CheckExitUnknown is used for when the check runs into a problem itself.
	CheckExitUnknown = 3
)
