package utils

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ExpandDuration expand duration string into seconds
func ExpandDuration(val string) (res float64, err error) {
	var num float64

	factors := []struct {
		suffix string
		factor float64
	}{
		{"ms", 0.001},
		{"s", 1},
		{"m", 60},
		{"h", 3600},
		{"d", 86400},
	}

	for _, f := range factors {
		if strings.HasSuffix(val, f.suffix) {
			num, err = strconv.ParseFloat(strings.TrimSuffix(val, f.suffix), 64)
			res = num * f.factor
			if err != nil {
				return 0, fmt.Errorf("expandDuration: %s", err.Error())
			}

			return res, nil
		}
	}
	if IsDigitsOnly(val) {
		res, err = strconv.ParseFloat(val, 64)

		if err != nil {
			return 0, fmt.Errorf("expandDuration: %s", err.Error())
		}

		return res, nil
	}

	return 0, fmt.Errorf("expandDuration: cannot parse duration, unknown format in %s", val)
}

// IsDigitsOnly returns true if string only contains numbers
func IsDigitsOnly(s string) bool {
	for _, c := range s {
		if !unicode.IsDigit(c) {
			return false
		}
	}

	return true
}

// IsFloatVal returns true if given val is a real float64 with fractions
// or false if value can be represented as int64
func IsFloatVal(val float64) bool {
	if strconv.FormatFloat(val, 'f', -1, 64) == fmt.Sprintf("%d", int64(val)) {
		return false
	}

	return true
}
