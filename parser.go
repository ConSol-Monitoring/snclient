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

type Threshold struct {
	name     string
	operator string
	value    string
	unit     string
}

type CheckData struct {
	warnThreshold *Threshold
	critThreshold *Threshold
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

func ParseArgs(args []string, data *CheckData) ([]Argument, error) {
	argList := make([]Argument, 0, len(args))
	for _, v := range args {
		split := strings.SplitN(v, "=", 2)
		switch split[0] {
		case "warn", "warning":
			thr, err := ParseThreshold(split[1])
			if err != nil {
				return nil, fmt.Errorf("threshold error: %s", err.Error())
			}
			data.warnThreshold = thr
		case "crit", "critical":
			thr, err := ParseThreshold(split[1])
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
		default:
			argList = append(argList, Argument{key: split[0], value: split[1]})
		}
	}

	return argList, nil
}

func ParseThreshold(threshold string) (*Threshold, error) {
	if threshold == "none" {
		return &Threshold{name: "none"}, nil
	}

	re := regexp.MustCompile(`^\s*([A-Za-z_]+)` +
		`\s*` +
		`(<=|>=|<|>|=|\!=|not like|is not|not|is|like)` +
		`\s*` +
		`(.*)$`)
	match := re.FindStringSubmatch(threshold)

	if len(match) == 0 {
		return nil, fmt.Errorf("cannot parse threshold: %s", threshold)
	}

	ret := Threshold{
		name:     match[1],
		operator: match[2],
		value:    strings.TrimSpace(match[3]),
	}

	switch {
	case strings.HasPrefix(ret.value, "'") && strings.HasSuffix(ret.value, "'"):
		ret.value = strings.TrimPrefix(ret.value, "'")
		ret.value = strings.TrimSuffix(ret.value, "'")
	case strings.HasPrefix(ret.value, `"`) && strings.HasSuffix(ret.value, `"`):
		ret.value = strings.TrimPrefix(ret.value, `"`)
		ret.value = strings.TrimSuffix(ret.value, `"`)
	}

	unitRe := regexp.MustCompile(`^(\d+\.\d+|\d+)\s*(\D+)$`)
	unitMatch := unitRe.FindStringSubmatch(ret.value)

	if len(unitMatch) != 0 {
		ret.value = unitMatch[1]
		ret.unit = unitMatch[2]
	}

	if ret.unit == "%" {
		ret.name += "_pct"
	}

	return &ret, nil
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
