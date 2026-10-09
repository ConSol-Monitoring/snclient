package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckEventlog(t *testing.T) {
	snc := Agent{}
	// match nothing
	res := snc.RunCheck("check_eventlog", []string{"filter=id < 0"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(
		t,
		`^OK - No entries found`,
		string(res.BuildPluginOutput()),
		"output matches",
	)
}

// Tests only the builder function for WHERE clause of the WMI query
// does not check the produced events
func TestCheckEventlogQueryWhereLines(t *testing.T) {
	tests := []struct {
		test     string
		check    CheckEventlog
		expected []string
		errMsg   string
	}{
		{
			test:     "no filters",
			check:    CheckEventlog{},
			expected: []string{},
		},
		{
			test:     "single event code",
			check:    CheckEventlog{eventCodesFilters: []string{"100"}},
			expected: []string{"EventCode = 100"},
		},
		{
			test:     "event codes with spaces",
			check:    CheckEventlog{eventCodesFilters: []string{" 100 , 101 ,"}},
			expected: []string{"(EventCode = 100 OR EventCode = 101)"},
		},
		{
			test:     "event codes max value",
			check:    CheckEventlog{eventCodesFilters: []string{"65535"}},
			expected: []string{"EventCode = 65535"},
		},
		{
			test:   "event codes out of range",
			check:  CheckEventlog{eventCodesFilters: []string{"65536"}},
			errMsg: "invalid event code \"65536\": must be an integer between 0 and 65535",
		},
		{
			test:   "negative event codes",
			check:  CheckEventlog{eventCodesFilters: []string{"-1"}},
			errMsg: "invalid event code \"-1\"",
		},
		{
			test:   "event codes empty",
			check:  CheckEventlog{eventCodesFilters: []string{" , "}},
			errMsg: "no event codes specified",
		},
		{
			test:     "event types by name",
			check:    CheckEventlog{eventTypesFilters: []string{"Warning, Success Audit"}},
			expected: []string{"(EventType = 2 OR EventType = 4)"},
		},
		{
			test:     "event types by alias",
			check:    CheckEventlog{eventTypesFilters: []string{"audit_failure"}},
			expected: []string{"EventType = 5"},
		},
		{
			test:     "event types by number",
			check:    CheckEventlog{eventTypesFilters: []string{"1,3"}},
			expected: []string{"(EventType = 1 OR EventType = 3)"},
		},
		{
			test:   "event types out of range",
			check:  CheckEventlog{eventTypesFilters: []string{"9"}},
			errMsg: "invalid event type 9: valid event types are",
		},
		{
			test:   "unknown event type",
			check:  CheckEventlog{eventTypesFilters: []string{"critical"}},
			errMsg: "invalid event type \"critical\"",
		},
		{
			test:     "source names with wildcards",
			check:    CheckEventlog{sourceNamesFilters: []string{"Microsoft-Windows-%,Service Control Manager"}},
			expected: []string{"(SourceName LIKE 'Microsoft-Windows-%' OR SourceName LIKE 'Service Control Manager')"},
		},
		{
			test:   "source names with quotes",
			check:  CheckEventlog{sourceNamesFilters: []string{"foo' OR 1=1--"}},
			errMsg: "contains one of the blacklisted characters",
		},
		{
			test:   "source names with control characters",
			check:  CheckEventlog{sourceNamesFilters: []string{"foo\x00bar"}},
			errMsg: "contains non printable characters",
		},
		{
			test: "all filters combined",
			check: CheckEventlog{
				eventCodesFilters:  []string{"100,101"},
				eventTypesFilters:  []string{"error"},
				sourceNamesFilters: []string{"MyApp"},
			},
			expected: []string{
				"(EventCode = 100 OR EventCode = 101)",
				"EventType = 1",
				"SourceName LIKE 'MyApp'",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.test, func(t *testing.T) {
			check := test.check
			whereLines, err := check.buildQueryWhereLines()
			if test.errMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.errMsg)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.expected, whereLines)
		})
	}
}
