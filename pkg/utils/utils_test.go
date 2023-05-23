package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUtilsExpandDuration(t *testing.T) {
	tests := []struct {
		in  string
		res float64
	}{
		{"2d", 86400 * 2},
		{"1m", 60},
		{"10s", 10},
		{"100ms", 0.1},
		{"100", 100},
	}

	for _, tst := range tests {
		res, err := ExpandDuration(tst.in)
		assert.NoError(t, err)
		assert.Equalf(t, tst.res, res, "ExpandDuration: %s", tst.in)
	}
}

func TestUtilsIsFloatVal(t *testing.T) {
	tests := []struct {
		in  float64
		res bool
	}{
		{1.00, false},
		{1.01, true},
		{5, false},
	}

	for _, tst := range tests {
		res := IsFloatVal(tst.in)
		assert.Equalf(t, tst.res, res, "IsFloatVal: %s", tst.in)
	}
}

func TestUtilsExecPath(t *testing.T) {
	execPath, _, _, err := GetExecutablePath()
	assert.NoErrorf(t, err, "GetExecutablePath works")

	assert.NotEmptyf(t, execPath, "GetExecutablePath")
}

func TestToPrecision(t *testing.T) {
	tests := []struct {
		in        float64
		precision int
		res       float64
	}{
		{1.001, 0, 1},
		{1.001, 3, 1.001},
		{1.0013, 3, 1.001},
	}

	for _, tst := range tests {
		res := ToPrecision(tst.in, tst.precision)
		assert.Equalf(t, tst.res, res, "ToPrecision: %v (precision: %d) -> %v", tst.in, tst.precision, res)
	}
}

func TestTokenizer(t *testing.T) {
	tests := []struct {
		in  string
		res []string
	}{
		{"", []string{""}},
		{"a bc d", []string{"a", "bc", "d"}},
		{"a 'bc' d", []string{"a", "'bc'", "d"}},
		{"a 'b c' d", []string{"a", "'b c'", "d"}},
		{`a "b'c" d`, []string{"a", `"b'c"`, "d"}},
		{`a 'b""c' d`, []string{"a", `'b""c'`, "d"}},
	}

	for _, tst := range tests {
		res := Tokenize(tst.in)
		assert.Equalf(t, tst.res, res, "Tokenize: %v -> %v", tst.in, res)
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		in  string
		res float64
	}{
		{"1.0", 1.0},
		{"0.1", 0.001},
		{"0.1.23", 0.001023},
	}

	for _, tst := range tests {
		res := ParseVersion(tst.in)
		assert.Equalf(t, tst.res, res, "ParseVersion: %v -> %v", tst.in, res)
	}
}

func TestDurationString(t *testing.T) {
	tests := []struct {
		in  time.Duration
		res string
	}{
		{time.Minute * 5, "00:05h"},
		{time.Hour * 5, "05:00h"},
		{time.Hour * 24, "1d 00:00h"},
		{time.Hour * 200, "8d 08:00h"},
		{time.Hour * 800, "4w 5d"},
		{time.Hour * 12345, "1y 21w"},
	}

	for _, tst := range tests {
		res := DurationString(tst.in)
		assert.Equalf(t, tst.res, res, "DurationString: %v -> %v", tst.in, res)
	}
}
