package snclient

import (
	"fmt"
	"strings"
)

// The generic filter is applied to the events after the wmi query in CheckData.Finalize,
// so pushing filter conditions into the query is only an optimization.
// The collected conditions are joined with OR, which selects a superset of the filter.
// It does not filter completely, only to a superset of the actual results,  therefore safe.
// Snclient filters the returned events again.

// buildFilterQueryWhereLine collects the values of all translatable filter conditions and
// joins them into a single WQL OR expression for the Win32_NTLogEvent query.
// An empty string is returned when a condition cannot be translated.
func (l *CheckEventlog) buildFilterQueryWhereLine(filter ConditionList) string {
	conditions := []string{}
	translatable := true

	err := filter.applyFuncToConditions(func(cond *Condition) error {
		// groups only hold sub conditions, they are visited by the recursion
		if cond.isNone || len(cond.group) > 0 {
			return nil
		}

		where, ok := l.conditionToWQL(cond)
		if !ok {
			translatable = false

			return nil
		}
		conditions = append(conditions, where...)

		return nil
	})
	if err != nil || !translatable {
		log.Tracef("filter conditions not pushed into the wmi query: %s", filter.String())

		return ""
	}

	if len(conditions) == 0 {
		return ""
	}

	return l.joinConditions(conditions, GroupOr)
}

// conditionToWQL translates a single filter condition into a list of WQL conditions.
// returns false if the condition cannot be translated.
func (l *CheckEventlog) conditionToWQL(cond *Condition) ([]string, bool) {
	switch cond.keyword {
	case "id":
		return l.eventCodeConditionsToWQL(cond)
	case "level":
		return l.levelConditionsToWQL(cond)
	case "source", "provider":
		return l.stringConditionsToWQL("SourceName", cond)
	case "file", "log":
		return l.stringConditionsToWQL("LogFile", cond)
	case "computer":
		return l.stringConditionsToWQL("ComputerName", cond)
	case "message":
		return l.stringConditionsToWQL("Message", cond)
	}

	return nil, false
}

// eventCodeConditionsToWQL translates id conditions into EventCode conditions.
func (l *CheckEventlog) eventCodeConditionsToWQL(cond *Condition) ([]string, bool) {
	values, ok := l.conditionEqualityValues(cond)
	if !ok {
		return nil, false
	}

	conditions := make([]string, 0, len(values))
	for _, value := range values {
		where, err := l.buildEventCodeCondition(value)
		if err != nil {
			return nil, false
		}
		conditions = append(conditions, where)
	}

	return conditions, true
}

// levelConditionsToWQL translates level conditions into Type conditions.
// level is the lowercase Type property.
// EventType must not be used instead, because Win32_NTLogEvent reports EventType 0 for some information events.
func (l *CheckEventlog) levelConditionsToWQL(cond *Condition) ([]string, bool) {
	values, ok := l.conditionEqualityValues(cond)
	if !ok {
		return nil, false
	}

	conditions := make([]string, 0, len(values))
	for _, value := range values {
		// a missing Type is an empty string in the check, but it never matches an empty wql string literal
		if value == "" || l.checkWQLString(value) != nil {
			return nil, false
		}
		conditions = append(conditions, fmt.Sprintf("Type = '%s'", strings.ToLower(value)))
	}

	return conditions, true
}

// stringConditionsToWQL translates equality and substring conditions into conditions for the given WQL string column.
func (l *CheckEventlog) stringConditionsToWQL(column string, cond *Condition) ([]string, bool) {
	switch cond.operator {
	case Equal, InList:
		values, ok := l.conditionEqualityValues(cond)
		if !ok {
			return nil, false
		}

		conditions := make([]string, 0, len(values))
		for _, value := range values {
			// missing wmi columns are empty strings in the check, but they never match an empty wql string literal
			if value == "" || l.checkWQLString(value) != nil {
				return nil, false
			}
			conditions = append(conditions, fmt.Sprintf("%s = '%s'", column, value))
		}

		return conditions, true
	case Contains, ContainsCase:
		value, ok := cond.value.(string)
		// like is a substring match in the check, so it translates into: '%value%'
		// wql cannot match missing values or the [ ] pattern characters with this translation.
		if !ok || value == "" || strings.ContainsAny(value, "[]") || l.checkWQLString(value) != nil {
			return nil, false
		}

		return []string{fmt.Sprintf("%s LIKE '%%%s%%'", column, value)}, true
	default:
		return nil, false
	}
}

// conditionEqualityValues returns the values of an "=" or "in" condition.
func (l *CheckEventlog) conditionEqualityValues(cond *Condition) ([]string, bool) {
	switch cond.operator {
	case Equal:
		value, ok := cond.value.(string)
		if !ok {
			return nil, false
		}

		return []string{value}, true
	case InList:
		values, ok := cond.value.([]string)
		if !ok || len(values) == 0 {
			return nil, false
		}

		return values, true
	default:
		return nil, false
	}
}
