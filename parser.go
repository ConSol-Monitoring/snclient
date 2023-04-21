package snclient

import (
	"regexp"
	"strings"
)

type Argument struct {
	key   string
	value string
}

type Treshold struct {
	name     string
	operator string
	value    string
	unit     string
}

type CheckData struct {
	warnTreshold Treshold
	critTreshold Treshold
	detailSyntax string
	topSyntax    string
	okSyntax     string
}

func ParseArgs(args []string, data *CheckData) []Argument {
	argList := make([]Argument, 0, len(args))
	for _, v := range args {
		split := strings.SplitN(v, "=", 2)
		switch split[0] {
		case "warn", "warning":
			data.warnTreshold = ParseTreshold(split[1])
		case "crit", "critical":
			data.critTreshold = ParseTreshold(split[1])
		case "detail-syntax":
			data.detailSyntax = split[1]
		case "top-syntax":
			data.topSyntax = split[1]
		case "ok-syntax":
			data.okSyntax = split[1]
		default:
			argList = append(argList, Argument{key: split[0], value: split[1]})
		}
	}

	return argList
}

func ParseTreshold(treshold string) Treshold {
	re := regexp.MustCompile(`([A-Za-z_]+)\s*(<=|>=|<|>|=|\!=|not like|not|is|like)\s*(\d+\.\d+|\d+|) *([A-Za-z0-9.%']+)?`)
	match := re.FindStringSubmatch(treshold)

	ret := Treshold{
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

	return ret
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
