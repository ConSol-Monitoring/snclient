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

	// event id filter conditions are pushed into the wmi query
	res = snc.RunCheck("check_eventlog", []string{
		"filter=id in (65535) and source like 'snclient-no-such-source'",
		"scan-range=-1h",
	})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(
		t,
		`^OK - No entries found`,
		string(res.BuildPluginOutput()),
		"output matches",
	)
}

// Tests only the translation of the generic filter into the WHERE clause of the WMI query.
func TestCheckEventlogFilterQueryWhereLine(t *testing.T) {
	attrs := NewCheckEventlog().Build().attributes

	tests := []struct {
		test     string
		filters  []string
		expected string
	}{
		{test: "no filter", filters: nil, expected: ""},
		{test: "none filter", filters: []string{"none"}, expected: ""},
		{test: "none filter combined with id filter", filters: []string{"none", "id = 1"}, expected: "EventCode = 1"},
		{
			test:     "none filter combined with id and source",
			filters:  []string{"none", "id in (823,824) and source like 'Firefox'"},
			expected: "(EventCode = 823 OR EventCode = 824 OR SourceName LIKE '%Firefox%')",
		},
		{test: "single event code", filters: []string{"id = 823"}, expected: "EventCode = 823"},
		{test: "event code zero", filters: []string{"id = 0"}, expected: "EventCode = 0"},
		{test: "event code max", filters: []string{"id = 65535"}, expected: "EventCode = 65535"},
		{
			test:     "event codes in list",
			filters:  []string{"id in (823,824,825,5173)"},
			expected: "(EventCode = 823 OR EventCode = 824 OR EventCode = 825 OR EventCode = 5173)",
		},
		{test: "event codes in list with invalid value", filters: []string{"id in (823,'abc')"}, expected: ""},
		{test: "event code out of range", filters: []string{"id = 65536"}, expected: ""},
		{test: "event code not numeric", filters: []string{"id = 'abc'"}, expected: ""},
		{test: "event code lower than", filters: []string{"id < 0"}, expected: ""},
		{test: "source equal", filters: []string{"source = 'MSSQL'"}, expected: "SourceName = 'MSSQL'"},
		{test: "provider equal", filters: []string{"provider = 'MSSQL'"}, expected: "SourceName = 'MSSQL'"},
		{
			test:     "source in list",
			filters:  []string{"source in ('MSSQL','SQL17')"},
			expected: "(SourceName = 'MSSQL' OR SourceName = 'SQL17')",
		},
		{test: "source like", filters: []string{"source like 'Firefox'"}, expected: "SourceName LIKE '%Firefox%'"},
		{test: "source slike", filters: []string{"source slike 'Firefox'"}, expected: "SourceName LIKE '%Firefox%'"},
		{test: "source with backslash", filters: []string{`source = 'foo\bar'`}, expected: ""},
		{test: "source unlike", filters: []string{"source unlike 'Firefox'"}, expected: ""},
		{test: "level equal", filters: []string{"level = 'error'"}, expected: "Type = 'error'"},
		{
			test:     "level in list",
			filters:  []string{"level in ('warning','error','critical')"},
			expected: "(Type = 'warning' OR Type = 'error' OR Type = 'critical')",
		},
		{test: "level like", filters: []string{"level like 'err'"}, expected: "Type LIKE '%err%'"},
		{test: "level not in", filters: []string{"level not in ('error')"}, expected: ""},
		{test: "level regex", filters: []string{"level ~ /err/"}, expected: ""},
		{test: "written greater", filters: []string{"written > 0"}, expected: ""},
		{
			test:     "event code and condition not translatable",
			filters:  []string{"id = 1 and written > 0"},
			expected: "EventCode = 1",
		},
		{
			test:     "event code or condition not translatable",
			filters:  []string{"id = 1 or written > 0"},
			expected: "",
		},
		{
			test:     "event codes combined with or",
			filters:  []string{"id = 1 or id = 2"},
			expected: "(EventCode = 1 OR EventCode = 2)",
		},
		{
			test:     "nested group",
			filters:  []string{"id = 1 and (id = 2 or id = 3)"},
			expected: "(EventCode = 1 OR EventCode = 2 OR EventCode = 3)",
		},
		{
			test:     "multiple top level filters",
			filters:  []string{"id = 1", "id = 2"},
			expected: "(EventCode = 1 OR EventCode = 2)",
		},
		{test: "multiple top level filters with one not translatable", filters: []string{"id = 1", "written > 0"}, expected: ""},
		{
			test:     "source and level combined with id",
			filters:  []string{"id = 903 and source like 'MSSQL' and level = 'error'"},
			expected: "(EventCode = 903 OR SourceName LIKE '%MSSQL%' OR Type = 'error')",
		},
		{
			test:     "message condition is ignored",
			filters:  []string{"id = 903 and message like 'foo'"},
			expected: "EventCode = 903",
		},
		{
			test:     "mssql example",
			filters:  []string{"id IN (823,824,825,5173) AND source like 'MSSQL' AND level IN ('critical','error')"},
			expected: "(EventCode = 823 OR EventCode = 824 OR EventCode = 825 OR EventCode = 5173 OR SourceName LIKE '%MSSQL%' OR Type = 'critical' OR Type = 'error')",
		},
	}

	for _, test := range tests {
		t.Run(test.test, func(t *testing.T) {
			filter := ConditionList{}
			for _, input := range test.filters {
				cond, err := NewCondition(input, &attrs)
				require.NoError(t, err)
				filter = append(filter, cond)
			}

			check := &CheckEventlog{}
			assert.Equal(t, test.expected, check.buildFilterQueryWhereLine(filter))
		})
	}
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
			test:     "multiple event code filters",
			check:    CheckEventlog{eventCodesFilters: []string{"100,101", "102"}},
			expected: []string{"(EventCode = 100 OR EventCode = 101)", "EventCode = 102"},
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
