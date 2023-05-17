package snclient

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
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

func (cr *CheckResult) Finalize(macros ...map[string]string) {
	if macros != nil {
		cr.Output = ReplaceMacros(cr.Output, macros...)
	}
	finalMacros := map[string]string{
		"status": cr.StateString(),
	}
	cr.Output = ReplaceMacros(cr.Output, finalMacros)
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

func (cr *CheckResult) BuildPluginOutput() []byte {
	output := []byte(cr.Output)
	if len(cr.Metrics) > 0 {
		perf := make([]string, 0, len(cr.Metrics))
		for _, m := range cr.Metrics {
			perf = append(perf, m.String())
		}
		output = append(output, '|')
		output = append(output, []byte(strings.Join(perf, " "))...)
	}

	return output
}

// CheckMetric contains a single performance value.
type CheckMetric struct {
	Name          string
	Unit          string
	Value         float64
	ThresholdName string
	Warning       []*Condition
	Critical      []*Condition
	Min           *float64
	Max           *float64
}

func (m *CheckMetric) String() string {
	var res bytes.Buffer

	res.WriteString(fmt.Sprintf("'%s'=%s%s", m.Name, strconv.FormatFloat(m.Value, 'f', -1, 64), m.Unit))

	res.WriteString(";")
	res.WriteString(m.ThresholdString(m.Warning))

	res.WriteString(";")
	res.WriteString(m.ThresholdString(m.Critical))

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

func (m *CheckMetric) ThresholdString(conditions []*Condition) string {
	if m.ThresholdName != "" {
		return ThresholdString(m.ThresholdName, conditions)
	}

	return ThresholdString(m.Name, conditions)
}
