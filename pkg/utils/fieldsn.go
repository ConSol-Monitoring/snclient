package utils

// Copyright (c) 2014 Kevin Wallace <kevin@pentabarf.net>
// Found here: https://github.com/kevinwallace/fieldsn
// Released under the MIT license
// XXX this implementation treats negative n as "return nil",
// unlike stdlib SplitN and friends, which treat it as "no limit"

import (
	"unicode"
)

// FieldsN is like strings.Fields, but returns at most n fields,
// and the nth field includes any whitespace at the end of the string.
func FieldsN(s string, n int) []string {
	return FieldsFuncN(s, unicode.IsSpace, n)
}

// FieldsFuncN is like strings.FieldsFunc, but returns at most n fields,
// and the nth field includes any runes at the end of the string normally excluded by f.
func FieldsFuncN(str string, fun func(rune) bool, max int) []string {
	if max <= 0 {
		return nil
	}

	fields := make([]string, 0, max)
	index := 0
	fieldStart := -1
	for idx, rune := range str {
		if fun(rune) {
			if fieldStart >= 0 {
				fields = append(fields, str[fieldStart:idx])
				index++
				fieldStart = -1
			}
		} else if fieldStart == -1 {
			fieldStart = idx
			if index+1 == max {
				break
			}
		}
	}
	if fieldStart >= 0 {
		fields = append(fields, str[fieldStart:])
	}

	return fields
}
