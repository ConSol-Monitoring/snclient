package snclient

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
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
		{"version not like  '1 2 3'", &Condition{keyword: "version", operator: ContainsNot, value: "1 2 3"}},
		{"state is not 0", &Condition{keyword: "state", operator: Unequal, value: "0"}},
		{"used gt 0", &Condition{keyword: "used", operator: Greater, value: "0"}},
		{"state not in ('started')", &Condition{keyword: "state", operator: NotInList, value: []string{"started"}}},
		{"state in ('a', 'b','c')", &Condition{keyword: "state", operator: InList, value: []string{"a", "b", "c"}}},
		{"state in ('a', 'b','c','d' )", &Condition{keyword: "state", operator: InList, value: []string{"a", "b", "c", "d"}}},
		{"state in ( 'a', 'b')", &Condition{keyword: "state", operator: InList, value: []string{"a", "b"}}},
		{
			"provider = 'abc' and id = 123 and message like 'foo'",
			&Condition{
				groupOperator: GroupAnd,
				group: []*Condition{
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
				group: []*Condition{
					{keyword: "provider", operator: Equal, value: "abc"},
					{
						groupOperator: GroupOr,
						group: []*Condition{
							{keyword: "id", operator: Equal, value: "123"},
							{keyword: "message", operator: Contains, value: "foo"},
						},
					},
				},
			},
		},
	} {
		cond, err := NewCondition(check.input)
		assert.NoErrorf(t, err, "ConditionParse should throw no error")
		assert.Equal(t, check.expect, cond, fmt.Sprintf("ConditionParse(%s) -> %v", check.input, check.expect))
	}
}

func TestConditionParseErrors(t *testing.T) {
	for _, check := range []struct {
		threshold string
		error     error
	}{
		{"val like", nil},
		{"val like '", nil},
		{"val like 'a", nil},
		{`val like "`, nil},
		{`val like "a`, nil},
		{"a > 5 and", nil},
		{"a >", nil},
		{"a 5", nil},
		{"> 5", nil},
		{"(a > 1 or b > 1", nil},
		{"((a > 1 or b > 1)", nil},
		{"a > 1 ) 1)", nil},
		{"state in ('a', 'b',)", nil},
		{"state in ('a', 'b',", nil},
		{"state in ('a', 'b'", nil},
		{"state in (", nil},
		{"a > 0 && b < 0 || x > 3", nil},
	} {
		cond, err := NewCondition(check.threshold)
		assert.Errorf(t, err, "ConditionParse should error")
		assert.Nilf(t, cond, fmt.Sprintf("ConditionParse(%s) errors should not return condition", check.threshold))
	}
}

func TestConditionCompare(t *testing.T) {
	for _, check := range []struct {
		threshold string
		key       string
		value     string
		expect    bool
	}{
		{"test > 5", "test", "2", false},
		{"test > 5", "test", "5.1", true},
		{"test >= 5", "test", "5.0", true},
		{"test < 5", "test", "5.1", false},
		{"test <= 5", "test", "5.0", true},
		{"test <= 5", "test", "5.1", false},
		{"test like abc", "test", "abcdef", true},
		{"test not like abc", "test", "abcdef", false},
		{"test like 'abc'", "test", "abcdef", true},
		{`test like "abc"`, "test", "abcdef", true},
		{`test in ('abc', '123', 'xyz')`, "test", "123", true},
		{`test in ('abc', '123', 'xyz')`, "test", "13", false},
		{`test not in ('abc', '123', 'xyz')`, "test", "123", false},
		{`test not in ('abc', '123', 'xyz')`, "test", "asd", true},
		{"test = 5", "test", "5", true},
		{"test = 5", "test", "5.0", true},
		{"test = 5.0", "test", "5", true},
		{"test = '123'", "test", "123", true},
		{"test != '123'", "test", "123", false},
		{"test regex 'a+'", "test", "aaaa", true},
		{"test regex 'a+'", "test", "bbbb", false},
		{"test !~ 'a+'", "test", "bbb", true},
		{"test !~ 'a+'", "test", "aa", false},
	} {
		threshold, err := NewCondition(check.threshold)
		assert.NoErrorf(t, err, "parsed threshold")
		assert.NotNilf(t, threshold, "parsed threshold")
		compare := map[string]string{check.key: check.value}
		assert.Equalf(t, check.expect, threshold.Match(compare), fmt.Sprintf("Compare(%s) -> (%v) %v", check.threshold, check.value, check.expect))
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
		threshold, err := NewCondition(check.threshold)
		assert.NoErrorf(t, err, "parsed threshold")
		assert.NotNilf(t, threshold, "parsed threshold")
		perfRange := ThresholdString(check.name, []*Condition{threshold})
		assert.Equalf(t, check.expect, perfRange, fmt.Sprintf("ThresholdString(%s) -> (%v) = %v", check.threshold, perfRange, check.expect))
	}
}
