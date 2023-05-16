package snclient

import (
	"regexp"
	"strings"
)

type Argument struct {
	key   string
	value string
}

// CheckData contains the runtime data of a generic check plugin
type CheckData struct {
	filter        []*Condition
	warnThreshold []*Condition
	critThreshold []*Condition
	detailSyntax  string
	topSyntax     string
	okSyntax      string
	listSyntax    string
	emptySyntax   string
	emptyState    int64
	details       map[string]string
	listData      []map[string]string
	result        CheckResult
}

// Check conditions against given data and set result state
func (cd *CheckData) Check(state int64, conditions []*Condition, data map[string]string) {
	// no need to escalate state anymore
	if cd.result.State >= state {
		return
	}

	for i := range conditions {
		if conditions[i].Match(data) {
			cd.result.State = state

			return
		}
	}
}

// Filter data map by conditions and return filtered list
func (cd *CheckData) Filter(conditions []*Condition, data []map[string]string) []map[string]string {
	result := make([]map[string]string, 0)

	for num := range data {
		matched := false
		for i := range conditions {
			if conditions[i].Match(data[num]) {
				break
			}
		}
		if matched {
			result = append(result, data[num])
		}
	}

	return result
}

func ParseStateString(state string) int64 {
	switch state {
	case "ok":
		return 0
	case "warn", "warning":
		return 1
	case "crit", "critical":
		return 2
	}

	return 3
}

// ParseArgs parses check arguments into the CheckData struct and returns all
// unknown options
func ParseArgs(args []string, data *CheckData) ([]Argument, error) {
	argList := make([]Argument, 0, len(args))
	for _, v := range args {
		split := strings.SplitN(v, "=", 2)
		switch split[0] {
		case "warn", "warning":
			cond, err := NewCondition(split[1])
			if err != nil {
				return nil, err
			}
			data.warnThreshold = append(data.warnThreshold, cond)
		case "crit", "critical":
			cond, err := NewCondition(split[1])
			if err != nil {
				return nil, err
			}
			data.critThreshold = append(data.critThreshold, cond)
		case "detail-syntax":
			data.detailSyntax = split[1]
		case "top-syntax":
			data.topSyntax = split[1]
		case "ok-syntax":
			data.okSyntax = split[1]
		case "empty-syntax":
			data.emptySyntax = split[1]
		case "empty-state":
			data.emptyState = ParseStateString(split[1])
		case "filter":
			cond, err := NewCondition(split[1])
			if err != nil {
				return nil, err
			}
			data.filter = append(data.filter, cond)
		default:
			argList = append(argList, Argument{key: split[0], value: split[1]})
		}
	}

	return argList, nil
}

func ParseSyntax(syntax string, data map[string]string) string {
	re := regexp.MustCompile(`[$%][{(](\w+)[})]`)

	matches := re.FindAllStringSubmatch(syntax, -1)

	for _, match := range matches {
		r := regexp.MustCompile(regexp.QuoteMeta(match[0]))
		syntax = r.ReplaceAllString(syntax, data[match[1]])
	}

	return syntax
}
