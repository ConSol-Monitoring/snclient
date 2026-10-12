package snclient

import (
	"fmt"
	"strings"
)

// The generic filter is applied to the events after the wmi query in CheckData.Finalize.
// Pushing filter conditions into the query is only an optimization,
// Every filter condition -> WQL translated condition must match a superset of the original condition.

// buildFilterQueryWhereLine translates the check filter into a WHERE expression for the Win32_NTLogEvent query.
// An empty string is returned if the filter cannot be translated safely.
func (l *CheckEventlog) buildFilterQueryWhereLine(filter ConditionList) string {
	if len(filter) == 0 {
		return ""
	}

	// top level filter conditions are combined with OR
	conditions := make([]string, 0, len(filter))
	for _, cond := range filter {
		where, ok := l.conditionToWQL(cond)
		if !ok {
			log.Tracef("filter condition not pushed into the wmi query: %s", cond.DetailedString())

			return ""
		}
		conditions = append(conditions, where)
	}

	return l.joinConditions(conditions, GroupOr)
}

// conditionToWQL translates a single filter condition into WQL.
// returns false if the condition cannot be translated safely.
func (l *CheckEventlog) conditionToWQL(cond *Condition) (string, bool) {
	if cond.isNone {
		return "", false
	}

	if len(cond.group) > 0 {
		return l.conditionGroupToWQL(cond)
	}

	return l.conditionLeafToWQL(cond)
}

// conditionGroupToWQL translates a group of conditions.
// Children without a translation are dropped from AND groups. That is ok, filtering to subset of the logical AND conditions is still an optimization.
// OR group can only be translated if all its children are translated.
func (l *CheckEventlog) conditionGroupToWQL(cond *Condition) (string, bool) {
	if cond.groupOperator != GroupAnd && cond.groupOperator != GroupOr {
		return "", false
	}

	conditions := make([]string, 0, len(cond.group))
	for _, sub := range cond.group {
		where, ok := l.conditionToWQL(sub)
		if !ok {
			if cond.groupOperator == GroupOr {
				log.Tracef("filter condition not pushed into the wmi query: %s", cond.DetailedString())

				return "", false
			}

			continue
		}
		conditions = append(conditions, where)
	}

	if len(conditions) == 0 {
		return "", false
	}

	return l.joinConditions(conditions, cond.groupOperator), true
}

// conditionLeafToWQL translates a single condition without logical groups.
func (l *CheckEventlog) conditionLeafToWQL(cond *Condition) (string, bool) {
	switch cond.keyword {
	case "id":
		return l.eventCodeConditionToWQL(cond)
	case "level":
		return l.levelConditionToWQL(cond)
	case "source", "provider":
		return l.stringConditionToWQL("SourceName", cond)
	case "file", "log":
		return l.stringConditionToWQL("LogFile", cond)
	case "computer":
		return l.stringConditionToWQL("ComputerName", cond)
	case "message":
		return l.stringConditionToWQL("Message", cond)
	}

	return "", false
}

// eventCodeConditionToWQL translates id conditions into EventCode conditions.
// Values that are no valid event code can never match and are skipped.
func (l *CheckEventlog) eventCodeConditionToWQL(cond *Condition) (string, bool) {
	values, ok := l.conditionEqualityValues(cond)
	if !ok {
		return "", false
	}

	conditions := make([]string, 0, len(values))
	for _, value := range values {
		where, err := l.buildEventCodeCondition(value)
		if err != nil {
			continue
		}
		conditions = append(conditions, where)
	}

	if len(conditions) == 0 {
		return "", false
	}

	return l.joinConditions(conditions, GroupOr), true
}

// levelConditionToWQL translates level conditions into EventType conditions.
// Levels that do not map to a windows event type can never match and are skipped.
func (l *CheckEventlog) levelConditionToWQL(cond *Condition) (string, bool) {
	values, ok := l.conditionEqualityValues(cond)
	if !ok {
		return "", false
	}

	conditions := make([]string, 0, len(values))
	for _, value := range values {
		eventType, err := l.parseEventType(value)
		if err != nil {
			continue
		}
		conditions = append(conditions, fmt.Sprintf("EventType = %d", eventType))
	}

	if len(conditions) == 0 {
		return "", false
	}

	return l.joinConditions(conditions, GroupOr), true
}

// stringConditionToWQL translates equality and substring conditions into conditions for the given WQL string column.
func (l *CheckEventlog) stringConditionToWQL(column string, cond *Condition) (string, bool) {
	switch cond.operator {
	case Equal, InList:
		values, ok := l.conditionEqualityValues(cond)
		if !ok {
			return "", false
		}

		conditions := make([]string, 0, len(values))
		for _, value := range values {
			// missing wmi columns are empty strings in the check, but they
			// never match an empty wql string literal
			if value == "" || l.checkWQLString(value) != nil {
				return "", false
			}
			conditions = append(conditions, fmt.Sprintf("%s = '%s'", column, value))
		}

		return l.joinConditions(conditions, GroupOr), true
	case Contains, ContainsCase:
		value, ok := cond.value.(string)
		// like is a substring match in the check, so it translates into
		// %value% . wql cannot match missing values or the [ ] pattern
		// characters with this translation.
		if !ok || value == "" || strings.ContainsAny(value, "[]") || l.checkWQLString(value) != nil {
			return "", false
		}

		return fmt.Sprintf("%s LIKE '%%%s%%'", column, value), true
	default:
		return "", false
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
