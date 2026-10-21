package snclient

import (
	"fmt"
	"strings"
	"unicode"
)

// The generic filter is applied to the events after the wmi query in CheckData.Finalize,
// so pushing filter conditions into the query is only an optimization. Event ids, event
// types and event sources are translated, all collected conditions are joined with OR,
// which selects a superset of the filter. Snclient filters the returned events again.

// buildFilterQueryWhereLine collects the translatable filter conditions and joins them
// into a single WQL OR expression for the Win32_NTLogEvent query.
// An empty string is returned when the filter cannot be translated safely.
func (l *CheckEventlog) buildFilterQueryWhereLine(filter ConditionList) string {
	conditions := []string{}
	translatable := true

	// top level filter conditions are combined with OR, filter=none conditions are ignored
	topLevel := 0
	for _, cond := range filter {
		if !cond.isNone {
			topLevel++
		}
	}
	hasOr := topLevel > 1

	err := filter.applyFuncToConditions(func(cond *Condition) error {
		// groups only hold sub conditions, they are visited by the recursion
		if cond.isNone {
			return nil
		}
		if len(cond.group) > 0 {
			hasOr = hasOr || cond.groupOperator == GroupOr

			return nil
		}

		where, ok := l.conditionLeafToWQL(cond)
		if ok {
			conditions = append(conditions, where...)

			return nil
		}
		translatable = false

		return nil
	})
	// without an OR in the filter all conditions have to match, so conditions that cannot
	// be translated can simply be ignored. With an OR they would have to be part of the
	// query, otherwise events matching only them would get lost.
	if err != nil || (hasOr && !translatable) {
		log.Tracef("filter conditions not pushed into the wmi query: %s", filter.String())

		return ""
	}

	if len(conditions) == 0 {
		return ""
	}

	return l.joinConditions(conditions)
}

// conditionLeafToWQL translates a single filter condition into WQL conditions.
func (l *CheckEventlog) conditionLeafToWQL(cond *Condition) ([]string, bool) {
	switch cond.keyword {
	case "id":
		return l.eventCodeConditionToWQL(cond)
	case "level":
		// level is the lowercase Type property
		return l.stringConditionToWQL("Type", cond)
	case "source", "provider":
		return l.stringConditionToWQL("SourceName", cond)
	}

	return nil, false
}

// eventCodeConditionToWQL translates the event ids of an "id" filter condition into EventCode conditions.
func (l *CheckEventlog) eventCodeConditionToWQL(cond *Condition) ([]string, bool) {
	values, ok := l.conditionValues(cond)
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

// stringConditionToWQL translates equality and substring conditions into conditions for the given WQL string column.
func (l *CheckEventlog) stringConditionToWQL(column string, cond *Condition) ([]string, bool) {
	switch cond.operator {
	case Equal, InList:
		values, ok := l.conditionValues(cond)
		if !ok {
			return nil, false
		}

		wqlConditions := make([]string, 0, len(values))
		for _, value := range values {
			// missing wmi columns are empty strings in the check, but they never match an empty wql string literal
			if value == "" || checkWQLString(value) != nil {
				return nil, false
			}
			wqlConditions = append(wqlConditions, fmt.Sprintf("%s = '%s'", column, value))
		}

		return wqlConditions, true
	case Contains, ContainsCase:
		value, ok := cond.value.(string)
		// like is a substring match in the check, so it translates into: '%value%'
		// wql cannot match missing values or the [ ] pattern characters with this translation.
		if !ok || value == "" || strings.ContainsAny(value, "[]") || checkWQLString(value) != nil {
			return nil, false
		}

		return []string{fmt.Sprintf("%s LIKE '%%%s%%'", column, value)}, true
	default:
		return nil, false
	}
}

// conditionValues returns the values of an "=" or "in" condition.
func (l *CheckEventlog) conditionValues(cond *Condition) ([]string, bool) {
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

// checkWQLString rejects values that would break or inject into a WQL string literal.
func checkWQLString(value string) error {
	// single quotes end the WQL string literal, backslashes are the WQL escape character
	if strings.ContainsAny(value, `'\`) {
		return fmt.Errorf("value %q contains a single quote or backslash", value)
	}
	if strings.ContainsFunc(value, func(r rune) bool { return !unicode.IsPrint(r) }) {
		return fmt.Errorf("value %q contains non printable characters", value)
	}

	return nil
}
