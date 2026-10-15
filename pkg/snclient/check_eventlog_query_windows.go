package snclient

// The generic filter is applied to the events after the wmi query in CheckData.Finalize,
// so pushing filter conditions into the query is only an optimization. Only event ids are
// translated, all collected conditions are joined with OR, which selects a superset of the
// filter. Snclient filters the returned events again.

// buildFilterQueryWhereLine collects the event ids of the check filter and joins them
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

		where, ok := l.eventCodeConditionToWQL(cond)
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

// eventCodeConditionToWQL translates the event ids of an "id" filter condition into EventCode conditions.
func (l *CheckEventlog) eventCodeConditionToWQL(cond *Condition) ([]string, bool) {
	if cond.keyword != "id" {
		return nil, false
	}

	values, ok := l.eventCodeValues(cond)
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

// eventCodeValues returns the event ids of an "=" or "in" condition.
func (l *CheckEventlog) eventCodeValues(cond *Condition) ([]string, bool) {
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
