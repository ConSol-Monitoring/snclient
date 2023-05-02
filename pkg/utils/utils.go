package utils

import (
	"fmt"
	"os"
	"path/filepath"
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
	return strconv.FormatFloat(val, 'f', -1, 64) != fmt.Sprintf("%d", int64(val))
}

// GetExecutablePath returns path to executable
func GetExecutablePath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("executable error: %s", err.Error())
	}

	executable, err = filepath.Abs(executable)
	if err != nil {
		return "", fmt.Errorf("abs error: %s", err.Error())
	}

	return filepath.Dir(executable), nil
}
