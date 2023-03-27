package snclient

import "fmt"

// CheckHandler handles a single check.
type CheckHandler interface {
	Check(Args []string) (*CheckResult, error)
}

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
