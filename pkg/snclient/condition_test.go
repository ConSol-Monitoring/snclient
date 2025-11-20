package snclient

import (
	"testing"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConditionParse(t *testing.T) {
	for _, check := range []struct {
		input  string
		expect *Condition
	}{
		{"none", &Condition{isNone: true}},
		{"load > 95%", &Condition{keyword: "load", operator: Greater, value: "95", unit: "%"}},
		{"used > 90GB", &Condition{keyword: "used", operator: Greater, value: "90000000000", unit: "B"}},
		{"used>90B", &Condition{keyword: "used", operator: Greater, value: "90", unit: "B"}},
		{"used >= 90GiB", &Condition{keyword: "used", operator: GreaterEqual, value: "96636764160", unit: "B"}},
		{"state = dead", &Condition{keyword: "state", operator: Equal, value: "dead"}},
		{"uptime < 180s", &Condition{keyword: "uptime", operator: Lower, value: "180", unit: "s"}},
		{"uptime < 2h", &Condition{keyword: "uptime", operator: Lower, value: "7200", unit: "s"}},
		{"uptime < 2H", &Condition{keyword: "uptime", operator: Lower, value: "7200", unit: "s"}},
		{"something < 3OP", &Condition{keyword: "something", operator: Lower, value: "3", unit: "OP"}},
		{"version not like  '1 2 3'", &Condition{keyword: "version", operator: ContainsNot, value: "1 2 3"}},
		{"state is not 0", &Condition{keyword: "state", operator: Unequal, value: "0"}},
		{"used gt 0", &Condition{keyword: "used", operator: Greater, value: "0"}},
		{"type = 'fixed'", &Condition{keyword: "type", operator: Equal, value: "fixed"}},
		{"type ='fixed'", &Condition{keyword: "type", operator: Equal, value: "fixed"}},
		{"type= 'fixed'", &Condition{keyword: "type", operator: Equal, value: "fixed"}},
		{"type='fixed'", &Condition{keyword: "type", operator: Equal, value: "fixed"}},
		{"command ~~ /ssh localhost/", &Condition{keyword: "command", operator: RegexMatchNoCase, value: "ssh localhost"}},
		{"command ~ /ssh localhost/i", &Condition{keyword: "command", operator: RegexMatchNoCase, value: "ssh localhost"}},
		{"command ~ /ssh localhost/", &Condition{keyword: "command", operator: RegexMatch, value: "ssh localhost"}},
		{"state not in ('started')", &Condition{keyword: "state", operator: NotInList, value: []string{"started"}}},
		{"state in ('a', 'b','c')", &Condition{keyword: "state", operator: InList, value: []string{"a", "b", "c"}}},
		{"state in ('a', 'b','c','d' )", &Condition{keyword: "state", operator: InList, value: []string{"a", "b", "c", "d"}}},
		{"state in ( 'a', 'b')", &Condition{keyword: "state", operator: InList, value: []string{"a", "b"}}},
		{
			"provider = 'abc' and id = 123 and message like 'foo'",
			&Condition{
				groupOperator: GroupAnd,
				group: ConditionList{
					{keyword: "provider", operator: Equal, value: "abc"},
					{keyword: "id", operator: Equal, value: "123"},
					{keyword: "message", operator: Contains, value: "foo"},
				},
			},
		},
		{
			"provider = 'abc' and (id = 123 or message like 'foo')",
			&Condition{
				groupOperator: GroupAnd,
				group: ConditionList{
					{keyword: "provider", operator: Equal, value: "abc"},
					{
						groupOperator: GroupOr,
						group: ConditionList{
							{keyword: "id", operator: Equal, value: "123"},
							{keyword: "message", operator: Contains, value: "foo"},
						},
					},
				},
			},
		},
	} {
		cond, err := NewCondition(check.input, nil, nil)
		check.expect.original = check.input
		require.NoErrorf(t, err, "ConditionParse should throw no error")
		assert.Equal(t, check.expect, cond, "ConditionParse(%s) -> %v", check.input, check.expect)
	}
}

func TestConditionStrings(t *testing.T) {
	for _, check := range []struct {
		input  ConditionList
		expect string
	}{
		{
			[]*Condition{{keyword: "state", operator: Equal, value: "ok"}},
			"state = ok",
		},
		{
			[]*Condition{{keyword: "drive", operator: Equal, value: "c:"}, {keyword: "drive", operator: Equal, value: "d:"}},
			"drive = c: or drive = d:",
		},
	} {
		str := check.input.String()
		assert.Equal(t, check.expect, str, "%v -> %s", check.input, check.expect)
	}
}

func TestConditionParseErrors(t *testing.T) {
	for _, check := range []struct {
		threshold string
	}{
		{"val like"},
		{"val like '"},
		{"val like 'a"},
		{`val like "`},
		{`val like "a`},
		{"a > 5 and"},
		{"a >"},
		{"a 5"},
		{"> 5"},
		{"(a > 1 or b > 1"},
		{"((a > 1 or b > 1)"},
		{"a > 1 ) 1)"},
		{"state in ('a', 'b',)"},
		{"state in ('a', 'b',"},
		{"state in ('a', 'b'"},
		{"state in ("},
		{"a > 0 && b < 0 || x > 3"},
	} {
		cond, err := NewCondition(check.threshold, nil, time.UTC)
		require.Errorf(t, err, "ConditionParse should error")
		assert.Nilf(t, cond, "ConditionParse(%s) errors should not return condition", check.threshold)
	}
}

func TestConditionCompare(t *testing.T) {
	for _, check := range []struct {
		threshold     string
		key           string
		value         string
		expect        bool
		deterministic bool
	}{
		{"test > 5", "test", "2", false, true},
		{"test > 5", "test", "5.1", true, true},
		{"test >= 5", "test", "5.0", true, true},
		{"test < 5", "test", "5.1", false, true},
		{"test <= 5", "test", "5.0", true, true},
		{"test <= 5", "test", "5.1", false, true},
		{"test like abc", "test", "abcdef", true, true},
		{"test not like abc", "test", "abcdef", false, true},
		{"test like 'abc'", "test", "abcdef", true, true},
		{`test like "abc"`, "test", "abcdef", true, true},
		{"test ilike 'AbC'", "test", "aBcdef", true, true},
		{"test not ilike 'AbC'", "test", "aBcdef", false, true},
		{`test in ('abc', '123', 'xyz')`, "test", "123", true, true},
		{`test in ('abc', '123', 'xyz')`, "test", "13", false, true},
		{`test not in ('abc', '123', 'xyz')`, "test", "123", false, true},
		{`test in('abc', '123', 'xyz')`, "test", "123", true, true},
		{`test not in ('abc', '123', 'xyz')`, "test", "asd", true, true},
		{`test not in('abc', '123', 'xyz')`, "test", "asd", true, true},
		{`test not in('abc','123','xyz')`, "test", "asd", true, true},
		{`test NOT IN('abc','123','xyz')`, "test", "asd", true, true},
		{"test = 5", "test", "5", true, true},
		{"test = 5", "test", "5.0", true, true},
		{"test = 5.0", "test", "5", true, true},
		{"test = '123'", "test", "123", true, true},
		{"test != '123'", "test", "123", false, true},
		{"test regex 'a+'", "test", "aaaa", true, true},
		{"test regex 'a+'", "test", "bbbb", false, true},
		{"test !~ 'a+'", "test", "bbb", true, true},
		{"test !~ 'a+'", "test", "aa", false, true},
		{"test ~~ 'a'", "test", "AAAA", true, true},
		{"test ~~ 'a'", "test", "BBBB", false, true},
		{"test ~ /a/i", "test", "AAAA", true, true},
		{"test ~ '/a/i'", "test", "AAAA", true, true},
		{"test !~ /a/i", "test", "aaa", false, true},
		{"test !~~ 'a'", "test", "AAAA", false, true},
		{"test !~~ 'a'", "test", "BBBB", true, true},
		{"'test space' > 5", "test space", "2", false, true},
		{"'test space' < 5", "test space", "2", true, true},
		{"unknown unlike blah", "test", "blah", false, false},
		{"unknown like blah", "test", "blah", false, false},
		{"unknown unlike blah or test like blah", "test", "blah", true, true},
		{"unknown like blah or test unlike blah", "test", "blah", false, false},
		{"unknown unlike blah and test like blah", "test", "blah", false, false},
		{"unknown like blah and test unlike blah", "test", "blah", false, true},
		{"test like 'blah'", "test", "blah", true, true},
		{"test ilike 'blah'", "test", "blah", true, true},
		{"test like 'Blah'", "test", "blah", true, true},
		{"test ilike 'Blah'", "test", "blah", true, true},
		{"test slike 'blah'", "test", "blah", true, true},
		{"test slike 'Blah'", "test", "blah", false, true},
		{"test like str(blah)", "test", "blah", true, true},
	} {
		threshold, err := NewCondition(check.threshold, nil, time.UTC)
		require.NoErrorf(t, err, "parsed threshold")
		assert.NotNilf(t, threshold, "parsed threshold")
		compare := map[string]string{check.key: check.value}
		res, ok := threshold.Match(compare)
		assert.Equalf(t, check.expect, res, "Compare(%s) -> (%v) %v", check.threshold, check.value, check.expect)
		assert.Equalf(t, check.deterministic, ok, "Compare(%s) -> determined: (%v) %v", check.threshold, check.value, check.deterministic)
	}
}

func TestConditionThresholdString(t *testing.T) {
	for _, check := range []struct {
		threshold string
		name      string
		expect    string
	}{
		{"test > 5", "test", "5"},
		{"test > 5 or test < 3", "test", "3:5"},
		{"test < 3 or test > 5", "test", "3:5"},
		{"test > 10 and test < 20", "test", "@10:20"},
		{"test < 20 and test > 10", "test", "@10:20"},
	} {
		threshold, err := NewCondition(check.threshold, nil, time.UTC)
		require.NoErrorf(t, err, "parsed threshold")
		assert.NotNilf(t, threshold, "parsed threshold")
		perfRange := ThresholdString([]string{check.name}, ConditionList{threshold}, convert.Num2String)
		assert.Equalf(t, check.expect, perfRange, "ThresholdString(%s) -> (%v) = %v", check.threshold, perfRange, check.expect)
	}
}

func TestConditionPreCheck(t *testing.T) {
	filterStr := `( name = 'xinetd' or name like 'other' ) and state = 'running'`

	for _, check := range []struct {
		filter    string
		entry     map[string]string
		expectPre bool
		expectFin bool
	}{
		{filterStr, map[string]string{"name": "xinetd", "state": "running"}, true, true},
		{filterStr, map[string]string{"name": "none", "state": "running"}, false, false},
		{filterStr, map[string]string{"test": "", "xyz": ""}, true, false},
	} {
		cond, err := NewCondition(check.filter, nil, time.UTC)
		require.NoError(t, err)
		chk := CheckData{}
		ok := chk.MatchMapCondition(ConditionList{cond}, check.entry, true)
		assert.Equalf(t, check.expectPre, ok, "pre check on %v returned: %v", check.entry, ok)

		ok = chk.MatchMapCondition(ConditionList{cond}, check.entry, false)
		assert.Equalf(t, check.expectPre, ok, "final check on %v returned: %v", check.entry, ok)

		// none filter
		cond, _ = NewCondition("none", nil, time.UTC)
		ok = chk.MatchMapCondition(ConditionList{cond}, check.entry, true)
		assert.Truef(t, ok, "none pre check on %v returned: %v", check.entry, ok)

		ok = chk.MatchMapCondition(ConditionList{cond}, check.entry, false)
		assert.Truef(t, ok, "none final check on %v returned: %v", check.entry, ok)
	}
}

func TestConditionAlias(t *testing.T) {
	filterStr := `( name = 'xinetd' or name like 'other' ) and state = 'started'`
	cond, err := NewCondition(filterStr, nil, time.UTC)
	require.NoError(t, err)

	check := &CheckData{
		filter: ConditionList{cond},
		conditionAlias: map[string]map[string]string{
			"state": {
				"started": "running",
			},
		},
	}
	check.applyConditionAlias()

	check.filter[0].original = "" // avoid shortcut String builder
	assert.Containsf(t, check.filter[0].String(), `state = running`, "filter condition replaced")
}

func TestConditionColAlias(t *testing.T) {
	filterStr := `( name = 'xinetd' and name unlike 'other' ) and state = 'started'`
	cond, err := NewCondition(filterStr, nil)
	require.NoError(t, err)

	check := &CheckData{
		filter: ConditionList{cond},
		conditionColAlias: map[string][]string{
			"name": {
				"name", "display",
			},
		},
	}
	check.applyConditionColAlias()

	check.filter[0].original = "" // avoid shortcut String builder
	assert.Equalf(t, `(((name = xinetd or display = xinetd) and (name unlike other and display unlike other)) and state = started)`, check.filter[0].String(), "filter condition replaced")
}

func TestConditionStrOp(t *testing.T) {
	input := "'blah' like str(Blah)"
	output := replaceStrOp(input)
	assert.Equal(t, `'blah' like 'Blah'`, output)

	input = "'blah' like str()"
	output = replaceStrOp(input)
	assert.Equal(t, `'blah' like ''`, output)

	input = "'blah' like str(a'b)"
	output = replaceStrOp(input)
	assert.Equal(t, `'blah' like 'a\'b'`, output)

	input = "'blah' like str(a\"'b)"
	output = replaceStrOp(input)
	assert.Equal(t, `'blah' like 'a"\'b'`, output)

	input = "'blah' like str(a'b'c)"
	output = replaceStrOp(input)
	assert.Equal(t, `'blah' like 'a\'b\'c'`, output)

	input = "( state not in ('running', 'oneshot', 'static') || active = 'failed' )  && preset != 'disabled'"
	output = replaceStrOp(input)
	assert.Equal(t, input, output)
}
