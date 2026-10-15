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

	// matching filter conditions are pushed into the wmi query
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
		{test: "none filter combined with filter", filters: []string{"none", "source like 'Firefox'"}, expected: "SourceName LIKE '%Firefox%'"},
		{
			test:     "none filter combined with two filters",
			filters:  []string{"none", "id = 1", "source like 'Firefox'"},
			expected: "(EventCode = 1 OR SourceName LIKE '%Firefox%')",
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
		{test: "event code invalid list", filters: []string{"id in ('abc')"}, expected: ""},
		{test: "event code lower than", filters: []string{"id < 0"}, expected: ""},
		{test: "source equal", filters: []string{"provider = 'MSSQL'"}, expected: "SourceName = 'MSSQL'"},
		{test: "source in list", filters: []string{"source in ('MSSQL','SQL17')"}, expected: "(SourceName = 'MSSQL' OR SourceName = 'SQL17')"},
		{test: "source like", filters: []string{"source like 'MSSQL'"}, expected: "SourceName LIKE '%MSSQL%'"},
		{test: "source slike", filters: []string{"source slike 'MSSQL'"}, expected: "SourceName LIKE '%MSSQL%'"},
		{test: "source unlike", filters: []string{"source unlike 'MSSQL'"}, expected: ""},
		{test: "source empty value", filters: []string{"source = ''"}, expected: ""},
		{test: "source with backslash", filters: []string{`source = 'foo\bar'`}, expected: ""},
		{test: "message like", filters: []string{"message like 'foo'"}, expected: "Message LIKE '%foo%'"},
		{test: "message like empty", filters: []string{"message like ''"}, expected: ""},
		{test: "message like pattern character", filters: []string{"message like 'a[b'"}, expected: ""},
		{test: "message regex", filters: []string{"message ~ /foo/"}, expected: ""},
		{test: "file equal", filters: []string{"file = 'Application'"}, expected: "LogFile = 'Application'"},
		{test: "log equal", filters: []string{"log = 'System'"}, expected: "LogFile = 'System'"},
		{test: "computer equal", filters: []string{"computer = 'host'"}, expected: "ComputerName = 'host'"},
		{test: "written greater", filters: []string{"written > 0"}, expected: ""},
		{test: "level equal", filters: []string{"level = 'error'"}, expected: "Type = 'error'"},
		{test: "level uppercase value", filters: []string{"level = 'Error'"}, expected: "Type = 'error'"},
		{test: "level unknown", filters: []string{"level = 'critical'"}, expected: "Type = 'critical'"},
		{
			test:     "level list",
			filters:  []string{"level in ('critical','error')"},
			expected: "(Type = 'critical' OR Type = 'error')",
		},
		{
			test:     "level list with information",
			filters:  []string{"level in ('warning','information')"},
			expected: "(Type = 'warning' OR Type = 'information')",
		},
		{test: "level empty value", filters: []string{"level = ''"}, expected: ""},
		{test: "level not in", filters: []string{"level not in ('error')"}, expected: ""},
		{test: "level regex", filters: []string{"level ~ /err/"}, expected: ""},
		{
			test:     "default filter",
			filters:  []string{"level in ('warning', 'error', 'critical')"},
			expected: "(Type = 'warning' OR Type = 'error' OR Type = 'critical')",
		},
		{
			test:     "and group",
			filters:  []string{"provider = 'x' and id = 903 and message like 'foo'"},
			expected: "(SourceName = 'x' OR EventCode = 903 OR Message LIKE '%foo%')",
		},
		{test: "and group with condition not translatable", filters: []string{"provider = 'x' and id < 0"}, expected: ""},
		{
			test:     "or group",
			filters:  []string{"provider = 'x' or id = 5"},
			expected: "(SourceName = 'x' OR EventCode = 5)",
		},
		{test: "or group with condition not translatable", filters: []string{"provider = 'x' or id < 0"}, expected: ""},
		{
			test:     "nested group",
			filters:  []string{"provider = 'abc' and (id = 123 or message like 'foo')"},
			expected: "(SourceName = 'abc' OR EventCode = 123 OR Message LIKE '%foo%')",
		},
		{
			test:     "multiple top level filters",
			filters:  []string{"id = 1", "id = 2"},
			expected: "(EventCode = 1 OR EventCode = 2)",
		},
		{test: "multiple top level filters with one not translatable", filters: []string{"id = 1", "id < 0"}, expected: ""},
		{
			test:     "all translatable conditions combined",
			filters:  []string{"id IN (823,824) AND source like 'MSSQL' AND source like 'SQL17' AND level IN ('critical','error')"},
			expected: "(EventCode = 823 OR EventCode = 824 OR SourceName LIKE '%MSSQL%' OR SourceName LIKE '%SQL17%' OR Type = 'critical' OR Type = 'error')",
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
