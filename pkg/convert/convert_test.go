package convert

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertFloat64E(t *testing.T) {
	tests := []struct {
		in  interface{}
		res float64
		err bool
	}{
		{1.5, 1.5, false},
		{"1.5", 1.5, false},
		{"1", 1, false},
		{"1e7", 1e7, false},
		{"", 0, true},
		{"abc", 0, true},
	}

	for _, tst := range tests {
		res, err := Float64E(tst.in)
		if tst.err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		assert.InDeltaf(t, tst.res, res, 0.00001, "Float64E: %s", tst.in)
	}
}

func TestConvertBoolE(t *testing.T) {
	tests := []struct {
		in  interface{}
		res bool
		err bool
	}{
		{true, true, false},
		{false, false, false},
		{"yes", true, false},
		{"no", false, false},
		{"No", false, false},
		{"Enabled", true, false},
		{"nope", false, true},
	}

	for _, tst := range tests {
		res, err := BoolE(tst.in)
		if tst.err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		assert.Equalf(t, tst.res, res, "BoolE: %v -> %v", tst.in, res)
	}
}

func TestConvertVersionF64(t *testing.T) {
	tests := []struct {
		in  interface{}
		res float64
		err bool
	}{
		{1.5, 1.5, false},
		{"1.5", 1.5, false},
		{"1", 1, false},
		{"7.3.4", 7.34, false},
		{"7.3-4", 7.34, false},
		{"10.0.22631.2715", 10.0226312715, false},
		{"v0.15.0065", 0.150065, false},
		{"10.0.22631.2715 Build 22631.2715", 10.0226312715, false},
		{"", 0, true},
		{"abc", 0, true},
	}

	for _, tst := range tests {
		res, err := VersionF64E(tst.in)
		if tst.err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		assert.InDeltaf(t, tst.res, res, 0.00001, "VersionF64E: %s", tst.in)
	}
}

func TestNum2String(t *testing.T) {
	tests := []struct {
		in  interface{}
		res string
		err bool
	}{
		{1.00, "1", false},
		{"100", "100", false},
		{"1.50", "1.5", false},
		{"abc", "", true},
		{"10737418240", "10737418240", false},
		{"1.5e4", "15000", false},
	}

	for _, tst := range tests {
		res, err := Num2StringE(tst.in)
		if tst.err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		assert.Equalf(t, tst.res, res, "Num2StringE: %T(%v) -> %v", tst.in, tst.in, res)
	}
}
