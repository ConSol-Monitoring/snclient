package snclient

import (
	"strings"
)

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

func (cr *CheckResult) EscalateStatus(state int64) {
	if state > cr.State {
		cr.State = state
	}
}

func (cr *CheckResult) BuildPluginOutput() []byte {
	output := []byte(cr.Output)
	if len(cr.Metrics) > 0 {
		perf := make([]string, 0, len(cr.Metrics))
		for _, m := range cr.Metrics {
			perf = append(perf, m.String())
		}
		output = append(output, ' ', '|')
		output = append(output, []byte(strings.Join(perf, " "))...)
	}

	return output
}
