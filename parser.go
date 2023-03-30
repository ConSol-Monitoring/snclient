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

func ParseArgs(args []string) []Argument {
	argList := make([]Argument, 0, len(args))
	for _, v := range args {
		split := strings.SplitN(v, "=", 2)
		argList = append(argList, Argument{key: split[0], value: split[1]})
	}

	return argList
}

func ParseTreshold(treshold string) Treshold {
	re := regexp.MustCompile(`([A-Za-z_]+)\s*(<=|>=|<|>|=|\!=|not|is)\s*(\d+|\d+\.\d+|) *([A-Za-z%]+)?`)
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
