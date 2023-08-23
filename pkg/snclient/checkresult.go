package snclient

import (
	"regexp"
	"strconv"
	"strings"

	"pkg/utils"
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

func (cr *CheckResult) ApplyPerfConfig(perfCfg []PerfConfig) error {
	tweakedMetrics := []*CheckMetric{}
	for i := range cr.Metrics {
		metric := cr.Metrics[i]
		found := false
		for i := range perfCfg {
			perf := perfCfg[i]
			if perf.Match(metric.Name) {
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
		if len(output) > 0 {
			output = append(output, ' ')
		}
		output = append(output, '|')
		output = append(output, []byte(strings.Join(perf, " "))...)
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
	for _, line := range strings.Split(cr.Output, "\n") {
		// get first pipe which is not escaped
		pipeIndex := cr.findPipeIndex(line)
		if pipeIndex == -1 {
			trimmedOutput = append(trimmedOutput, line)

			continue
		}

		rawPerfData := strings.TrimSpace(line[pipeIndex+1:])

		// remove perf data from normal output
		line := strings.TrimSpace(line[:pipeIndex])
		trimmedOutput = append(trimmedOutput, line)

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
			min, err := strconv.ParseFloat(values[3], 64)
			if err != nil {
				log.Debugf("broken performance data, no cannot parse float in %s: %s", raw, err.Error())
			} else {
				metric.Min = &min
			}
		}

		// max
		if len(values) > 4 && values[4] != "" {
			max, err := strconv.ParseFloat(values[4], 64)
			if err != nil {
				log.Debugf("broken performance data, no cannot parse float in %s: %s", raw, err.Error())
			} else {
				metric.Max = &max
			}
		}

		metrics = append(metrics, metric)
	}

	return metrics
}
