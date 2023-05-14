package snclient

import (
	"fmt"
	"regexp"
	"strings"
)

type Argument struct {
	key   string
	value string
}

type CheckData struct {
	filter        []*Condition
	warnThreshold *Threshold // TODO: make them list of conditions
	critThreshold *Threshold // TODO: make them list of conditions
	detailSyntax  string
	topSyntax     string
	okSyntax      string
	emptySyntax   string
	emptyState    int64
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
			thr, err := ThresholdParse(split[1])
			if err != nil {
				return nil, fmt.Errorf("threshold error: %s", err.Error())
			}
			data.warnThreshold = thr
		case "crit", "critical":
			thr, err := ThresholdParse(split[1])
			if err != nil {
				return nil, fmt.Errorf("threshold error: %s", err.Error())
			}
			data.critThreshold = thr
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
			cond, err := ConditionParse(split[1])
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
