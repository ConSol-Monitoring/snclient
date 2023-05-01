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
	regexDigit                 = `(-?\d+(\.\d+)?)`
	regexOutsideZeroToX        = regexp.MustCompile(fmt.Sprintf(`^%s$`, regexDigit))
	regexOutsideXToInf         = regexp.MustCompile(fmt.Sprintf(`^%s:$`, regexDigit))
	regexOutsideMinusInfToX    = regexp.MustCompile(fmt.Sprintf(`^~:%s$`, regexDigit))
	regexInsideOutsideLow2High = regexp.MustCompile(fmt.Sprintf(`^(@?)%s:%s$`, regexDigit, regexDigit))
	minFloat64                 = float64(math.MinInt64)

	// ErrFirstBiggerThenSecond this error is thrown when the first number is bigger then the second
	ErrFirstBiggerThenSecond = errors.New("first argument is bigger then second")
)

// String prints the Threshold
func (t Threshold) String() string {
	return t.input
}

// NewThreshold constructs a new Threshold from string, returns an Threshold if possible else nil and an error
func NewThreshold(def string) (*Threshold, error) {
	def = strings.TrimSpace(def)
	if def == "" {
		return nil, fmt.Errorf("empty threshold given")
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
	if outsideLow2High := regexInsideOutsideLow2High.FindAllStringSubmatch(def, -1); len(outsideLow2High) == 1 {
		if len(outsideLow2High[0]) != 6 {
			return nil, fmt.Errorf("threshold parse error")
		}
		low, err := strconv.ParseFloat(outsideLow2High[0][2], 64)
		if err != nil {
			return nil, fmt.Errorf("threshold parse error: %s", err.Error())
		}
		high, err := strconv.ParseFloat(outsideLow2High[0][4], 64)
		if err != nil {
			return nil, fmt.Errorf("threshold parse error: %s", err.Error())
		}
		if low > high {
			return nil, ErrFirstBiggerThenSecond
		}

		return &Threshold{input: def, lower: low, upper: high, outside: outsideLow2High[0][1] != "@"}, nil
	}

	return nil, fmt.Errorf("threshold syntax not supported: %s", def)
}

// CheckValue tests if the given value fulfills the thresholds
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
