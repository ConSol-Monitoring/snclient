package threshold

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Threshold contains the threshold logic: https://www.monitoring-plugins.org/doc/guidelines.html#THRESHOLDFORMAT
type Threshold struct {
	input   string
	lower   float64
	upper   float64
	outside bool
}

var (
	regexDigit              = `(-?\d+(\.\d+)?)`
	regexOutsideZeroToX     = regexp.MustCompile(fmt.Sprintf(`^%s$`, regexDigit))
	regexOutsideXToInf      = regexp.MustCompile(fmt.Sprintf(`^%s:$`, regexDigit))
	regexOutsideMinusInfToX = regexp.MustCompile(fmt.Sprintf(`^~:%s$`, regexDigit))
	regexInsideOutsideXToY  = regexp.MustCompile(fmt.Sprintf(`^(@?)%s:%s$`, regexDigit, regexDigit))
	minFloat64              = float64(math.MinInt64)

	// ErrFirstBiggerThenSecond this error is thrown when the first number is bigger then the second
	ErrFirstBiggerThenSecond = errors.New("First argument is bigger then second")
)

// String prints the Threshold
func (t Threshold) String() string {
	return t.input
}

// NewThreshold constructs a new Threshold from string, returns an Threshold if possible else nil and an error
func NewThreshold(def string) (*Threshold, error) {
	def = strings.TrimSpace(def)
	if def == "" {
		return nil, nil
	}
	if outsideZeroToX := regexOutsideZeroToX.FindString(def); outsideZeroToX != "" {
		if x, err := strconv.ParseFloat(outsideZeroToX, 64); err == nil {
			return &Threshold{input: def, lower: 0, upper: x, outside: true}, nil
		}
	}
	if outsideXtoInf := regexOutsideXToInf.FindStringSubmatch(def); len(outsideXtoInf) == 3 {
		if x, err := strconv.ParseFloat(outsideXtoInf[1], 64); err == nil {
			return &Threshold{input: def, lower: x, upper: math.MaxFloat64, outside: true}, nil
		}
	}
	if outsideMinusInfToX := regexOutsideMinusInfToX.FindStringSubmatch(def); len(outsideMinusInfToX) == 3 {
		if x, err := strconv.ParseFloat(outsideMinusInfToX[1], 64); err == nil {
			return &Threshold{input: def, lower: minFloat64, upper: x, outside: true}, nil
		}
	}
	if outsideXToY := regexInsideOutsideXToY.FindAllStringSubmatch(def, -1); len(outsideXToY) == 1 && len(outsideXToY[0]) == 6 {
		if x, err := strconv.ParseFloat(outsideXToY[0][2], 64); err == nil {
			if y, err := strconv.ParseFloat(outsideXToY[0][4], 64); err == nil {
				if x > y {
					return nil, ErrFirstBiggerThenSecond
				}
				if outsideXToY[0][1] == "@" {
					return &Threshold{input: def, lower: x, upper: y, outside: false}, nil
				} else {
					return &Threshold{input: def, lower: x, upper: y, outside: true}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("This threshold syntax is not supported: %s", def)
}

// IsValueOK tests if the given value fulfills the Thresholds
// false: value is critical/warning
// true: value is ok
func (t *Threshold) CheckValue(value float64) bool {
	if t.outside {
		if value < t.lower || value > t.upper {
			return false
		}
	} else {
		return !(value >= t.lower && value <= t.upper)
	}
	return true
}
