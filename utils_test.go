package snclient

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
