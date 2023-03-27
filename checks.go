package snclient

import "fmt"

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

// CheckResult is the result of a single check run.
type CheckResult struct {
	State   int64
	Output  string
	Metrics []*CheckMetric
}

// CheckMetric contains a single performance value.
type CheckMetric struct {
	Name     string
	Unit     string
	Value    float64
	Warning  CheckThreshold
	Critical CheckThreshold
	Min      float64
	Max      float64
}

func (m *CheckMetric) BuildNaemonString() string {
	return (fmt.Sprintf("'%s%s'=%f;;;%f;%f", m.Name, m.Unit, m.Value, m.Min, m.Max))
}

// CheckThreshold defines a threshold range.
type CheckThreshold struct {
	Low  float64
	High float64
}