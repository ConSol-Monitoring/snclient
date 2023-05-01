package snclient

import (
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

func ParseArgs(args []string, data *CheckData) []Argument {
	argList := make([]Argument, 0, len(args))
	for _, v := range args {
		split := strings.SplitN(v, "=", 2)
		switch split[0] {
		case "warn", "warning":
			data.warnThreshold = ParseThreshold(split[1])
		case "crit", "critical":
			data.critThreshold = ParseThreshold(split[1])
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

	return argList
}

func ParseThreshold(threshold string) *Threshold {
	if threshold == "none" {
		return &Threshold{name: "none"}
	}

	re := regexp.MustCompile(`([A-Za-z_]+)\s*(<=|>=|<|>|=|\!=|not like|not|is|like)\s*(\d+\.\d+|\d+|) *'?([A-Za-z0-9.%']+)?`)
	match := re.FindStringSubmatch(threshold)

	ret := Threshold{
		name:     match[1],
		operator: match[2],
	}

	if match[3] != "" {
		ret.value = match[3]
		ret.unit = match[4]
	} else {
		ret.value = match[4]
	}

	if ret.unit == "%" {
		ret.name += "_pct"
	}

	return &ret
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
