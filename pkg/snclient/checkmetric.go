package snclient

import (
	"bytes"
	"fmt"
	"maps"
	"strconv"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/humanize"
	"github.com/consol-monitoring/snclient/pkg/utils"
)

// CheckMetric contains a single performance value.
type CheckMetric struct {
	Name          string        // Name as used in the perf data string
	Unit          string        // Unit of the value
	Value         any           // Current value
	ThresholdName string        // if set, this will be added to the data before checking a conditions
	Warning       ConditionList // threshold used for warnings
	WarningStr    *string       // set warnings from string
	Critical      ConditionList // threshold used for critical
	CriticalStr   *string       // set critical from string
	Min           *float64
	Max           *float64
	PerfConfig    *PerfConfig       // apply perf tweaks
	Entry         map[string]string // entry that this metric is generated from
}

// generates a naemon like string, including the perfdata
func (m *CheckMetric) String() string {
	var res bytes.Buffer

	name := m.Name
	if m.PerfConfig != nil {
		// Suffix replaces the current name
		if m.PerfConfig.Suffix != "" {
			name = m.PerfConfig.Suffix
		}

		if m.PerfConfig.Prefix != "" {
			name = fmt.Sprintf("%s%s", m.PerfConfig.Prefix, name)
		}
	}

	// Unknown value
	if fmt.Sprintf("%v", m.Value) == "U" {
		return fmt.Sprintf("'%s'=U", name)
	}

	num, unit := m.tweakedNum(m.Value)
	fmt.Fprintf(&res, "'%s'=%s%s", name, num, unit)

	res.WriteString(";")
	if m.WarningStr != nil {
		res.WriteString(*m.WarningStr)
	} else {
		res.WriteString(m.ThresholdString(m.Warning))
	}

	res.WriteString(";")
	if m.CriticalStr != nil {
		res.WriteString(*m.CriticalStr)
	} else {
		res.WriteString(m.ThresholdString(m.Critical))
	}

	res.WriteString(";")
	if m.Min != nil {
		if m.PerfConfig != nil {
			num, _ := m.tweakedNum(*m.Min)
			res.WriteString(num)
		} else {
			res.WriteString(strconv.FormatFloat(*m.Min, 'f', -1, 64))
		}
	}

	res.WriteString(";")
	if m.Max != nil {
		if m.PerfConfig != nil {
			num, _ := m.tweakedNum(*m.Max)
			res.WriteString(num)
		} else {
			res.WriteString(strconv.FormatFloat(*m.Max, 'f', -1, 64))
		}
	}

	resStr := res.String()
	// strip trailing semicolons
	for strings.HasSuffix(resStr, ";") {
		resStr = strings.TrimSuffix(resStr, ";")
	}

	return resStr
}

// return name but apply tweaks from perf-config before
func (m *CheckMetric) tweakedName() (name string) {
	name = m.Name

	if m.PerfConfig == nil {
		return name
	}

	// Suffix replaces the current name
	if m.PerfConfig.Suffix != "" {
		name = m.PerfConfig.Suffix
	}

	if m.PerfConfig.Prefix != "" {
		name = fmt.Sprintf("%s%s", m.PerfConfig.Prefix, name)
	}

	return name
}

// tweakedNum applies perf-config tweaks to a given number and returns the formatted number and unit.
// It handles multiplication by a magic factor, conversion to percentages, and unit conversions
func (m *CheckMetric) tweakedNum(rawNum any) (num, unit string) {
	str := fmt.Sprintf("%v", rawNum)
	if str == "U" {
		return str, ""
	}
	if m.PerfConfig == nil {
		return convert.Num2String(rawNum), m.Unit
	}

	if m.PerfConfig.Magic != 1 {
		val := convert.Float64(rawNum)
		rawNum = val * m.PerfConfig.Magic
	}

	if m.PerfConfig.Unit == "%" && m.Min != nil && m.Max != nil && *m.Max > *m.Min {
		// convert into percent
		val := convert.Float64(rawNum)
		perc := ((val - *m.Min) / (*m.Max - *m.Min)) * 100

		return convert.Num2String(perc), m.PerfConfig.Unit
	}

	if m.Unit == "" && m.PerfConfig.Unit != "" {
		m.Unit = m.PerfConfig.Unit
	}

	switch m.Unit {
	case "B":
		// convert bytes
		num := humanize.BytesUnitF(convert.UInt64(rawNum), m.PerfConfig.Unit, 3)

		return convert.Num2String(num), m.PerfConfig.Unit
	case "s":
		// convert seconds
		num := utils.TimeUnitF(uint64(convert.Float64(rawNum)), m.PerfConfig.Unit, 1)

		return convert.Num2String(num), m.PerfConfig.Unit
	}

	return convert.Num2String(rawNum), m.Unit
}

// Generate a string to be used in naemon like perfdata output about this threshold
// if a metric has a reference to its originating entry, the conditions will check if it is in their skip list
func (m *CheckMetric) ThresholdString(conditions ConditionList) string {
	conv := func(rawNum any) string {
		num, _ := m.tweakedNum(rawNum)

		return num
	}

	namesToUseWhenBuildingPerfString := []string{}

	for name := range maps.Keys(m.BuildCheckData()) {
		namesToUseWhenBuildingPerfString = append(namesToUseWhenBuildingPerfString, name)
	}

	return ThresholdString(namesToUseWhenBuildingPerfString, conditions, conv)
}

// When performing checks against conditions, a data map of type map[string]string is required
// Metric objects build their data using their Name and if specified, threshold value
func (m *CheckMetric) BuildCheckData() (data map[string]string) {
	// build up a data map[string]string as condition.Match function requires it as an argument
	data = map[string]string{
		m.Name: fmt.Sprintf("%v", m.Value),
	}
	if m.ThresholdName != "" {
		data[m.ThresholdName] = fmt.Sprintf("%v", m.Value)
	}

	return data
}

// Metrics have a defined way for specifying key / values
//
//nolint:dogsled // only need corrects list here
func (m *CheckMetric) CheckForThresholds(conditionList *ConditionList) (result bool) {
	data := m.BuildCheckData()

	corrects, _, _, _ := conditionList.performMatches(data, true)

	return len(corrects) > 1
}
