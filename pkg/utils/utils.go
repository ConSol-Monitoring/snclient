package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"github.com/kdar/factorlog"
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

// ToPrecision converts float64 to given precision, ex.: 5.12345 -> 5.1
func ToPrecision(val float64, precision int) float64 {
	format := fmt.Sprintf("%%.%df", precision)
	valStr := fmt.Sprintf(format, val)
	short, _ := strconv.ParseFloat(valStr, 64)

	return short
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

func ReadPid(pidfile string) (int, error) {
	dat, err := os.ReadFile(pidfile)
	if err != nil {
		return 0, fmt.Errorf("read %s failed: %s", pidfile, err.Error())
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(dat)))
	if err != nil {
		return 0, fmt.Errorf("read %s failed: %s", pidfile, err.Error())
	}

	return pid, nil
}

func LogThreadDump(log *factorlog.FactorLog) {
	buf := make([]byte, 1<<16)

	if n := runtime.Stack(buf, true); n < len(buf) {
		buf = buf[:n]
	}

	log.Errorf("ThreadDump:\n%s", buf)
}

// Tokenize returns list of string tokens
func Tokenize(str string) []string {
	return (TokenizeBy(str, " \t\n\r"))
}

// TokenizeBy returns list of string tokens separated by any char in separator
func TokenizeBy(str string, separator string) []string {
	var tokens []string

	inQuotes := false
	inDbl := false
	token := make([]rune, 0)
	for _, char := range str {
		switch {
		case char == '"':
			if !inQuotes {
				inDbl = !inDbl
			}
			token = append(token, char)
		case char == '\'':
			if !inDbl {
				inQuotes = !inQuotes
			}
			token = append(token, char)
		case strings.ContainsRune(separator, char):
			switch {
			case inQuotes, inDbl:
				token = append(token, char)
			case len(token) > 0:
				tokens = append(tokens, string(token))
				token = make([]rune, 0)
			}
		default:
			token = append(token, char)
		}
	}
	tokens = append(tokens, string(token))

	return tokens
}