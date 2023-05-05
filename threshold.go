package snclient

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"pkg/utils"

	"github.com/dustin/go-humanize"
)

type Threshold struct {
	name     string
	operator string
	value    string
	unit     string
}

func ThresholdParse(threshold string) (*Threshold, error) {
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

	switch ret.unit {
	case "KB", "MB", "GB", "TB", "PB":
		value, _ := humanize.ParseBytes(ret.value + ret.unit)
		ret.value = strconv.FormatUint(value, 10)
		ret.unit = "B"
	case "m", "h", "d":
		value, _ := utils.ExpandDuration(ret.value)
		ret.value = strconv.FormatFloat(value, 'f', 0, 64)
		ret.unit = "s"
	case "%":
		ret.name += "_pct"
	}

	return &ret, nil
}

func (t *Threshold) String() string {
	return t.value
}
