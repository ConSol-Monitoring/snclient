package snclient

import (
	"regexp"
	"strconv"
	"strings"
)

type Treshold struct {
	name     string
	operator string
	value    int
	unit     string
}

func ParseArgs(args []string) []map[string]string {
	var argList []map[string]string
	for _, v := range args {

		split := strings.SplitN(v, "=", 2)
		argList = append(argList, map[string]string{"key": split[0], "value": split[1]})

	}

	return argList
}

func ParseTreshold(treshold string) Treshold {
	operatorRe := regexp.MustCompile("[<|>|=|!=]")
	split := operatorRe.Split(treshold, -1)

	valueRe := regexp.MustCompile("[0-9]+")
	unitRe := regexp.MustCompile("\\D+")
	value, _ := strconv.Atoi(string(valueRe.Find([]byte(split[1]))))

	return Treshold{
		name:     strings.TrimSpace(split[0]),
		operator: string(operatorRe.Find([]byte(treshold))),
		value:    value,
		unit:     string(unitRe.Find([]byte(split[1]))),
	}
}
