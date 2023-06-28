package snclient

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"pkg/convert"
)

// CheckMetric contains a single performance value.
type CheckMetric struct {
	Name          string
	Unit          string
	Value         interface{}
	ThresholdName string
	Warning       []*Condition
	Critical      []*Condition
	Min           *float64
	Max           *float64
}

func (m *CheckMetric) String() string {
	var res bytes.Buffer

	// Unknown value
	if fmt.Sprintf("%v", m.Value) == "U" {
		return fmt.Sprintf("'%s'=U", m.Name)
	}

	res.WriteString(fmt.Sprintf("'%s'=%s%s", m.Name, convert.Num2String(m.Value), m.Unit))

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
