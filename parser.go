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

	var argList []Argument

	for _, v := range args {

		split := strings.SplitN(v, "=", 2)
		argList = append(argList, Argument{key: split[0], value: split[1]})

	}

	return argList

}

func ParseTreshold(treshold string) Treshold {

	re := regexp.MustCompile("([A-Za-z]+)\\s*(<=|>=|<|>|=|\\!=)\\s*(\\d+|\\d+\\.\\d+|) *([A-Za-z%]+)?")
	match := re.FindStringSubmatch(treshold)

	name := match[1]
	value := ""
	unit := ""

	if match[3] != "" {
		value = match[3]
		unit = match[4]
	} else {
		value = match[4]
	}

	if unit == "%" {
		name += "_pct"
	}

	return Treshold{
		name:     name,
		operator: match[2],
		value:    value,
		unit:     unit,
	}

}
