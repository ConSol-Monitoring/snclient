package snclient

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/utils"
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

var reValuesUnit = regexp.MustCompile(`^([0-9.]+)(.*?)$`)

// CheckResult is the result of a single check run.
type CheckResult struct {
	State   int64          // naemon exit code: OK=0, Warning=1, Critical=2, Unknown=3
	Output  string         // plugin output, should be human readable
	Metrics []*CheckMetric // performance data metrics
	Raw     *CheckData     // reference to the original check data, for use in inventory and other checks
	Details string         // additional details that should be printed on a new line after the main output, e.g. for showing top consuming processes
}

func (cr *CheckResult) Finalize(timezone *time.Location, macros ...map[string]string) {
	macroSet := make([]map[string]string, 0, len(macros)+1)
	macroSet = append(macroSet, map[string]string{
		"status": cr.StateString(),
	})
	macroSet = append(macroSet, macros...)

	output, err := ReplaceTemplate(cr.Output, timezone, macroSet...)
	if err != nil {
		log.Debugf("replacing template failed: %s: %s", cr.Output, err.Error())
	}
	cr.Output = output
	details, err := ReplaceConditionals(cr.Details, macroSet...)
	if err != nil {
		log.Debugf("replacing details template failed: %s: %s", cr.Details, err.Error())
	}
	cr.Details = details

	// replace macros twice to make nested assignments work
	for range []int64{1, 2} {
		cr.Output = ReplaceMacros(cr.Output, timezone, macroSet...)
	}
	cr.Details = ReplaceMacros(cr.Details, timezone, macroSet...)
}

func (cr *CheckResult) ApplyPerfConfig(perfCfg []PerfConfig) error {
	tweakedMetrics := []*CheckMetric{}
	for _, metric := range cr.Metrics {
		found := false
		for i := range perfCfg {
			perf := perfCfg[i]
			if perf.Match(metric.Name) {
				log.Tracef("perfdata '%s' matched perf-config: %s", metric.Name, perf.Raw)
				found = true
				if perf.Ignore {
					break
				}

				metric.PerfConfig = &perf
				tweakedMetrics = append(tweakedMetrics, metric)

				break
			}

			if found {
				break
			}
		}

		// no tweak config found, simply pass it through
		if !found {
			tweakedMetrics = append(tweakedMetrics, metric)
		}
	}
	cr.Metrics = tweakedMetrics

	return nil
}

func (cr *CheckResult) ApplyPerfSyntax(perfSyntax string, timezone *time.Location) {
	if perfSyntax == "" || perfSyntax == "%(key)" {
		return
	}

	for i := range cr.Metrics {
		metric := cr.Metrics[i]
		macros := map[string]string{
			"key": metric.Name,
		}
		// save original name, so thresholds can be added properly later
		if metric.ThresholdName == "" {
			metric.ThresholdName = metric.Name
		}
		metric.Name = ReplaceMacros(perfSyntax, timezone, macros)
	}
}

func (cr *CheckResult) StateString() string {
	return convert.StateString(cr.State)
}

func (cr *CheckResult) EscalateStatus(state int64) {
	if state > cr.State {
		cr.State = state
	}
}

// BuildOutputString returns the output string with details if available
func (cr *CheckResult) BuildOutputString() string {
	if cr.Details != "" {
		return cr.Output + "\n" + cr.Details
	}

	return cr.Output
}

// BuildPluginOutput returns the output as bytes including metrics
func (cr *CheckResult) BuildPluginOutput() []byte {
	output := []byte(cr.BuildOutputString())
	if len(cr.Metrics) > 0 {
		lines := bytes.Split(output, []byte("\n"))
		firstLine := lines[0]
		perf := make([]string, 0, len(cr.Metrics))
		for _, m := range cr.Metrics {
			perf = append(perf, m.String())
		}
		if len(firstLine) > 0 {
			firstLine = append(firstLine, ' ')
		}
		firstLine = append(firstLine, '|')
		firstLine = append(firstLine, []byte(strings.Join(perf, " "))...)
		lines[0] = firstLine
		output = bytes.Join(lines, []byte("\n"))
	}

	return output
}

// ParsePerformanceDataFromOutputCond checks the 'ignore perfdata' and extracts performance data unless disabled
func (cr *CheckResult) ParsePerformanceDataFromOutputCond(command string, conf *ConfigSection) {
	ignorePerfdata, ok, err := conf.GetBool("ignore perfdata")
	switch {
	case err != nil:
		log.Errorf("%s: ignore perfdata: %s", command, err.Error())

		return
	case ok && ignorePerfdata:
		return
	}

	cr.ParsePerformanceDataFromOutput()
}

// Parse performance data from the Output and put them into Metrics
func (cr *CheckResult) ParsePerformanceDataFromOutput() {
	if cr.Metrics == nil {
		cr.Metrics = []*CheckMetric{}
	}
	trimmedOutput := []string{}
	// parse output line by line and extract metrics
	for line := range strings.SplitSeq(cr.Output, "\n") {
		// get first pipe which is not escaped
		pipeIndex := cr.findPipeIndex(line)
		if pipeIndex == -1 {
			trimmedOutput = append(trimmedOutput, line)

			continue
		}

		rawPerfData := strings.TrimSpace(line[pipeIndex+1:])

		// remove perf data from normal output
		trimmedOutput = append(trimmedOutput, strings.TrimSpace(line[:pipeIndex]))

		metrics := cr.extractMetrics(rawPerfData)
		cr.Metrics = append(cr.Metrics, metrics...)
	}

	cr.Output = strings.Join(trimmedOutput, "\n")
}

func (cr *CheckResult) findPipeIndex(str string) int {
	escaped := false

	for i, char := range str {
		switch {
		case char == '\\':
			escaped = true
		case char == '|' && !escaped:
			return i
		default:
			escaped = false
		}
	}

	return -1
}

func (cr *CheckResult) extractMetrics(str string) []*CheckMetric {
	metrics := []*CheckMetric{}

	for _, raw := range utils.Tokenize(str) {
		metric := &CheckMetric{}
		splitted := strings.SplitN(raw, "=", 2)
		if len(splitted) < 2 {
			log.Debugf("broken performance data, no = found in %s", raw)

			continue
		}

		// metrics name
		name, err := utils.TrimQuotes(splitted[0])
		if err != nil {
			log.Debugf("broken performance data, no = found in %s", raw)

			continue
		}
		metric.Name = name

		values := strings.SplitN(splitted[1], ";", 5)

		// value and unit
		valUnits := reValuesUnit.FindStringSubmatch(values[0])
		if len(valUnits) > 2 {
			metric.Value = valUnits[1]
			metric.Unit = valUnits[2]
		} else {
			metric.Value = values[0]
		}

		// warning threshold
		if len(values) > 1 && values[1] != "" {
			metric.WarningStr = &values[1]
		}

		// critical threshold
		if len(values) > 2 && values[2] != "" {
			metric.CriticalStr = &values[2]
		}

		// min
		if len(values) > 3 && values[3] != "" {
			minV, err := strconv.ParseFloat(values[3], 64)
			if err != nil {
				log.Debugf("broken performance data, no cannot parse float in %s: %s", raw, err.Error())
			} else {
				metric.Min = &minV
			}
		}

		// max
		if len(values) > 4 && values[4] != "" {
			maxV, err := strconv.ParseFloat(values[4], 64)
			if err != nil {
				log.Debugf("broken performance data, no cannot parse float in %s: %s", raw, err.Error())
			} else {
				metric.Max = &maxV
			}
		}

		metrics = append(metrics, metric)
	}

	return metrics
}
