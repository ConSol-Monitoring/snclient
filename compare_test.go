package snclient

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompare(t *testing.T) {
	t.Parallel()

	for _, check := range []struct {
		threshold string
		key       string
		value     string
		expect    bool
	}{
		{"test > 5", "test", "2", false},
		{"test > 5", "test", "5.1", true},
		{"test >= 5", "test", "5.0", true},
		{"test like abc", "test", "abcdef", true},
		{"test not like abc", "test", "abcdef", false},
		{"test like 'abc'", "test", "abcdef", true},
		{`test like "abc"`, "test", "abcdef", true},
	} {
		threshold, err := ThresholdParse(check.threshold)
		assert.NoErrorf(t, err, "parsed threshold")
		assert.NotNilf(t, threshold, "parsed threshold")
		compare := map[string]string{check.key: check.value}
		assert.Equalf(t, check.expect, CompareMetrics(compare, threshold), fmt.Sprintf("Compare(%s) -> %v", check.threshold, check.expect))
	}
}
