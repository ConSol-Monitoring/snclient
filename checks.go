package snclient

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"pkg/threshold"
)

type CheckEntry struct {
	Name    string
	Handler CheckHandler
}

var AvailableChecks = make(map[string]CheckEntry)

const (
	// CheckExitOK is used for normal exits.
	CheckExitOK = int64(0)

	// CheckExitWarning is used for warnings.
	CheckExitWarning = int64(1)

	// CheckExitCritical is used for critical errors.
	CheckExitCritical = int64(2)

	// CheckExitUnknown is used for when the check runs into a problem itself.
	CheckExitUnknown = int64(3)
)

// CheckResult is the result of a single check run.
type CheckResult struct {
	State   int64
	Output  string
	Metrics []*CheckMetric
}

func (cr *CheckResult) StateString() string {
	switch cr.State {
	case 0:
		return "OK"
	case 1:
		return "WARNING"
	case 2:
		return "CRITICAL"
	}

	return "UNKNOWN"
}

func (cr *CheckResult) replaceOutputVariables() {
	cr.Output = strings.ReplaceAll(cr.Output, "${status}", cr.StateString())
	cr.Output = strings.ReplaceAll(cr.Output, "${status_lc}", strings.ToLower(cr.StateString()))
}

// CheckMetric contains a single performance value.
type CheckMetric struct {
	Name     string
	Unit     string
	Value    float64
	Warning  *threshold.Threshold
	Critical *threshold.Threshold
	Min      *float64
	Max      *float64
}

func (m *CheckMetric) String() string {
	var res bytes.Buffer

	res.WriteString(fmt.Sprintf("'%s'=%s%s", m.Name, strconv.FormatFloat(m.Value, 'f', -1, 64), m.Unit))

	res.WriteString(";")
	if m.Warning != nil {
		res.WriteString(m.Warning.String())
	}

	res.WriteString(";")
	if m.Critical != nil {
		res.WriteString(m.Critical.String())
	}

	res.WriteString(";")
	if m.Min != nil {
		res.WriteString(strconv.FormatFloat(*m.Min, 'f', -1, 64))
	}

	res.WriteString(";")
	if m.Max != nil {
		res.WriteString(strconv.FormatFloat(*m.Max, 'f', -1, 64))
	}

	resStr := res.String()
	// strip trailing semicolons
	for strings.HasSuffix(resStr, ";") {
		resStr = strings.TrimSuffix(resStr, ";")
	}

	return resStr
}
