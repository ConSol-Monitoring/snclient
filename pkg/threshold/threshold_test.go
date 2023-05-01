package threshold

import (
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewThreshold(t *testing.T) {
	t.Parallel()

	stringToThreshold := []struct {
		input     string
		threshold *Threshold
		err       error
	}{
		{"-3.4", &Threshold{input: "-3.4", lower: 0, upper: -3.4, outside: true}, nil},
		{" 3.4", &Threshold{input: "3.4", lower: 0, upper: 3.4, outside: true}, nil},
		{"3", &Threshold{input: "3", lower: 0, upper: 3, outside: true}, nil},
		{"-3", &Threshold{input: "-3", lower: 0, upper: -3, outside: true}, nil},
		{"foo", nil, errors.New("")},
		{"3,4", nil, errors.New("")},

		{" -3.4:", &Threshold{input: "-3.4:", lower: -3.4, upper: math.MaxFloat64, outside: true}, nil},
		{"3.4:", &Threshold{input: "3.4:", lower: 3.4, upper: math.MaxFloat64, outside: true}, nil},
		{"-3:", &Threshold{input: "-3:", lower: -3, upper: math.MaxFloat64, outside: true}, nil},
		{"3:", &Threshold{input: "3:", lower: 3, upper: math.MaxFloat64, outside: true}, nil},
		{"3,1:", nil, errors.New("")},

		{"~:-3.4 ", &Threshold{input: "~:-3.4", lower: minFloat64, upper: -3.4, outside: true}, nil},
		{"~:3.4", &Threshold{input: "~:3.4", lower: minFloat64, upper: 3.4, outside: true}, nil},
		{"~:-3", &Threshold{input: "~:-3", lower: minFloat64, upper: -3, outside: true}, nil},
		{" ~:3", &Threshold{input: "~:3", lower: minFloat64, upper: 3, outside: true}, nil},
		{"~:3,1", nil, errors.New("")},

		{"-1.2:-3.4", nil, ErrFirstBiggerThenSecond},
		{"3:2", nil, ErrFirstBiggerThenSecond},
		{"1.2:3.4", &Threshold{input: "1.2:3.4", lower: 1.2, upper: 3.4, outside: true}, nil},
		{"-1.2:3.4", &Threshold{input: "-1.2:3.4", lower: -1.2, upper: 3.4, outside: true}, nil},
		{"-3.4:-1.2", &Threshold{input: "-3.4:-1.2", lower: -3.4, upper: -1.2, outside: true}, nil},
		{"1.2:3", &Threshold{input: "1.2:3", lower: 1.2, upper: 3, outside: true}, nil},
		{"1:3", &Threshold{input: "1:3", lower: 1, upper: 3, outside: true}, nil},
		{"1:3.4", &Threshold{input: "1:3.4", lower: 1, upper: 3.4, outside: true}, nil},
		{"1,2:3,4", nil, errors.New("")},

		{"@-1.2:-3.4", nil, ErrFirstBiggerThenSecond},
		{" @3:2", nil, ErrFirstBiggerThenSecond},
		{"@1.2:3.4", &Threshold{input: "@1.2:3.4", lower: 1.2, upper: 3.4, outside: false}, nil},
		{"@-1.2:3.4 ", &Threshold{input: "@-1.2:3.4", lower: -1.2, upper: 3.4, outside: false}, nil},
		{"@-3.4:-1.2", &Threshold{input: "@-3.4:-1.2", lower: -3.4, upper: -1.2, outside: false}, nil},
		{"@1.2:3", &Threshold{input: "@1.2:3", lower: 1.2, upper: 3, outside: false}, nil},
		{"@1:3", &Threshold{input: "@1:3", lower: 1, upper: 3, outside: false}, nil},
		{"@1:3.4", &Threshold{input: "@1:3.4", lower: 1, upper: 3.4, outside: false}, nil},
		{"@1,2:3,4", nil, errors.New("")},
	}

	for _, data := range stringToThreshold {
		tGot, err := NewThreshold(data.input)
		if data.err != nil {
			assert.Errorf(t, err, "threshold results in error")
		} else {
			assert.NoErrorf(t, err, "threshold results not in error")
		}
		assert.Equalf(t, tGot, data.threshold, "threshold ok")
	}
}

func TestThreshold_IsValueOK(t *testing.T) {
	t.Parallel()

	thresholdBorders := []struct {
		threshold string
		value     float64
		expected  bool
	}{
		{"10", -1, false},
		{"10", 0, true},
		{"10", 1, true},
		{"10", 10, true},
		{"10", 11, false},

		{"10:", -1, false},
		{"10:", 9, false},
		{"10:", 10, true},
		{"10:", 11, true},

		{"~:10", 11, false},
		{"~:10", 10, true},
		{"~:10", 9, true},
		{"~:10", -1, true},

		{"10:20", -1, false},
		{"10:20", 9, false},
		{"10:20", 10, true},
		{"10:20", 11, true},
		{"10:20", 19, true},
		{"10:20", 20, true},
		{"10:20", 21, false},

		{"@10:20", -1, true},
		{"@10:20", 9, true},
		{"@10:20", 10, false},
		{"@10:20", 11, false},
		{"@10:20", 19, false},
		{"@10:20", 20, false},
		{"@10:20", 21, true},
	}

	for _, data := range thresholdBorders {
		th, err := NewThreshold(data.threshold)
		assert.NoErrorf(t, err, "no error expected")

		result := th.CheckValue(data.value)
		assert.Equalf(t, data.expected, result, "range ok")
	}
}
