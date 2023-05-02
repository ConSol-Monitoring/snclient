package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUtilsExpandDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in  string
		res float64
	}{
		{"2d", 86400 * 2},
		{"1m", 60},
		{"10s", 10},
		{"100ms", 0.1},
	}

	for _, tst := range tests {
		res, err := ExpandDuration(tst.in)
		assert.NoError(t, err)
		assert.Equalf(t, tst.res, res, "ExpandDuration: %s", tst.in)
	}
}

func TestUtilsIsFloatVal(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	execPath, err := GetExecutablePath()
	assert.NoErrorf(t, err, "GetExecutablePath works")

	assert.NotEmptyf(t, execPath, "GetExecutablePath")
}
