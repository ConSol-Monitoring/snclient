package snclient

import (
	"fmt"
	"strings"
)

type Argument struct {
	key   string
	value string
}

// CheckData contains the runtime data of a generic check plugin
type CheckData struct {
	noCopy          noCopy
	debug           string
	defaultFilter   string
	conditionAlias  map[string]map[string]string // replacement map of equivalent condition values
	args            map[string]interface{}
	filter          []*Condition // if set, only show entries matching this filter set
	warnThreshold   []*Condition
	defaultWarning  string
	critThreshold   []*Condition
	defaultCritical string
	okThreshold     []*Condition
	detailSyntax    string
	topSyntax       string
	okSyntax        string
	emptySyntax     string
	emptyState      int64
	details         map[string]string
	listData        []map[string]string
	result          *CheckResult
}

func (cd *CheckData) Finalize() (*CheckResult, error) {
	defer restoreLogLevel()
	log.Tracef("finalize check results:")
	if cd.details == nil {
		cd.details = map[string]string{}
	}
	cd.Check(cd.details, cd.warnThreshold, cd.critThreshold, cd.okThreshold)
	log.Tracef("details: %v", cd.details)

	// apply final filter
	cd.listData = cd.Filter(cd.filter, cd.listData)

	if len(cd.listData) > 0 {
		log.Tracef("list data:")
		for _, l := range cd.listData {
			cd.Check(l, cd.warnThreshold, cd.critThreshold, cd.okThreshold)
			log.Tracef(" - %v", l)
		}
	}

	finalMacros := cd.buildListMacros()

	cd.Check(finalMacros, cd.warnThreshold, cd.critThreshold, cd.okThreshold)
	cd.setStateFromMaps(finalMacros)

	switch {
	case cd.result.Output != "":
		// already set, leave it
		return cd.result, nil
	case len(cd.filter) > 0 && len(cd.listData) == 0:
		cd.result.Output = cd.emptySyntax
		cd.result.State = cd.emptyState
	case cd.result.State == 0 && cd.okSyntax != "":
		cd.result.Output = cd.okSyntax
	default:
		cd.result.Output = cd.topSyntax
	}

	cd.result.Finalize(cd.details, finalMacros)

	return cd.result, nil
}

func (cd *CheckData) buildListMacros() map[string]string {
	list := []string{}
	okList := make([]string, 0)
	warnList := make([]string, 0)
	critList := make([]string, 0)
	for _, l := range cd.listData {
		expanded := ReplaceMacros(cd.detailSyntax, l)
		list = append(list, expanded)
		switch l["_state"] {
		case "0":
			okList = append(okList, expanded)
		case "1":
			warnList = append(warnList, expanded)
		case "2":
			critList = append(critList, expanded)
		}
	}

	problemList := make([]string, 0)
	problemList = append(problemList, critList...)
	problemList = append(problemList, warnList...)

	detailList := append(problemList, okList...)

	return map[string]string{
		"count":         fmt.Sprintf("%d", len(list)),
		"list":          strings.Join(list, ", "),
		"ok_count":      fmt.Sprintf("%d", len(okList)),
		"ok_list":       strings.Join(okList, ", "),
		"warn_count":    fmt.Sprintf("%d", len(warnList)),
		"warn_list":     strings.Join(warnList, ", "),
		"crit_count":    fmt.Sprintf("%d", len(critList)),
		"crit_list":     strings.Join(critList, ", "),
		"problem_count": fmt.Sprintf("%d", len(problemList)),
		"problem_list":  strings.Join(problemList, ", "),
		"detail_list":   strings.Join(detailList, ", "),
	}
}

func (cd *CheckData) setStateFromMaps(macros map[string]string) {
	switch macros["_state"] {
	case "1":
		cd.result.EscalateStatus(1)
	case "2":
		cd.result.EscalateStatus(2)
	case "3":
		cd.result.EscalateStatus(3)
	}

	switch {
	case macros["crit_count"] != "0":
		cd.result.EscalateStatus(2)
		macros["_state"] = "2"
	case macros["warn_count"] != "0":
		cd.result.EscalateStatus(1)
		macros["_state"] = "1"
	}

	cd.details["_state"] = fmt.Sprintf("%d", cd.result.State)
}

// Check conditions against given data and set result state
func (cd *CheckData) Check(data map[string]string, warnCond, critCond, okCond []*Condition) {
	data["_state"] = fmt.Sprintf("%d", CheckExitOK)

	for i := range warnCond {
		if warnCond[i].Match(data) {
			data["_state"] = fmt.Sprintf("%d", CheckExitWarning)
		}
	}

	for i := range critCond {
		if critCond[i].Match(data) {
			data["_state"] = fmt.Sprintf("%d", CheckExitCritical)
		}
	}

	for i := range okCond {
		if okCond[i].Match(data) {
			data["_state"] = fmt.Sprintf("%d", CheckExitOK)
		}
	}
}

// MatchFilter returns true if {name: value} matches any filter
func (cd *CheckData) MatchFilter(name, value string) bool {
	data := map[string]string{name: value}
	for _, cond := range cd.filter {
		if cond.isNone {
			return true
		}
		if cond.Match(data) {
			return true
		}
	}

	return false
}

// MatchMapCondition returns true listEntry matches filter
func (cd *CheckData) MatchMapCondition(conditions []*Condition, entry map[string]string) bool {
	for i := range conditions {
		if conditions[i].isNone {
			continue
		}
		if !conditions[i].Match(entry) {
			return false
		}
	}

	return true
}

// Filter data map by conditions and return filtered list
func (cd *CheckData) Filter(conditions []*Condition, data []map[string]string) []map[string]string {
	result := make([]map[string]string, 0)

	for num := range data {
		if cd.MatchMapCondition(conditions, data[num]) {
			result = append(result, data[num])
		}
	}

	return result
}

func (cd *CheckData) parseStateString(state string) int64 {
	switch state {
	case "0", "ok":
		return 0
	case "1", "warn", "warning":
		return 1
	case "2", "crit", "critical":
		return 2
	}

	return 3
}

// ParseArgs parses check arguments into the CheckData struct
// and returns all unknown options
func (cd *CheckData) ParseArgs(args []string) ([]Argument, error) {
	argList := make([]Argument, 0, len(args))
	for _, argExpr := range args {
		argExpr = cd.removeQuotes(argExpr)
		split := strings.SplitN(cd.removeQuotes(argExpr), "=", 2)
		if len(split) == 1 {
			split = append(split, "")
		}
		keyword := cd.removeQuotes(split[0])
		argValue := cd.removeQuotes(split[1])
		switch keyword {
		case "ok":
			cond, err := NewCondition(argValue)
			if err != nil {
				return nil, err
			}
			cd.okThreshold = append(cd.okThreshold, cond)
		case "warn", "warning":
			cond, err := NewCondition(argValue)
			if err != nil {
				return nil, err
			}
			cd.warnThreshold = append(cd.warnThreshold, cond)
		case "crit", "critical":
			cond, err := NewCondition(argValue)
			if err != nil {
				return nil, err
			}
			cd.critThreshold = append(cd.critThreshold, cond)
		case "debug":
			cd.debug = argValue
			if cd.debug == "" {
				cd.debug = "debug"
			}
		case "detail-syntax":
			cd.detailSyntax = argValue
		case "top-syntax":
			cd.topSyntax = argValue
		case "ok-syntax":
			cd.okSyntax = argValue
		case "empty-syntax":
			cd.emptySyntax = argValue
		case "empty-state":
			cd.emptyState = cd.parseStateString(argValue)
		case "filter":
			cond, err := NewCondition(argValue)
			if err != nil {
				return nil, err
			}
			cd.filter = append(cd.filter, cond)
		default:
			if arg, ok := cd.args[keyword]; ok {
				switch argRef := arg.(type) {
				case *[]string:
					*argRef = append(*argRef, argValue)
				case *string:
					*argRef = argValue
				default:
					log.Errorf("unsupported args type: %T in %s", argRef, argExpr)
				}
			} else {
				argList = append(argList, Argument{key: keyword, value: argValue})
			}
		}
	}

	err := cd.setFallbacks()
	if err != nil {
		return nil, err
	}

	cd.applyConditionAlias()

	// increase logLevel temporary if debug arg is set
	raiseLogLevel(cd.debug)

	return argList, nil
}

func (cd *CheckData) removeQuotes(str string) string {
	switch {
	case strings.HasPrefix(str, "'") && strings.HasSuffix(str, "'"):
		str = strings.TrimPrefix(str, "'")
		str = strings.TrimSuffix(str, "'")

		return str
	case strings.HasPrefix(str, `"`) && strings.HasSuffix(str, `"`):
		str = strings.TrimPrefix(str, `"`)
		str = strings.TrimSuffix(str, `"`)

		return str
	}

	return str
}

func (cd *CheckData) setFallbacks() error {
	if len(cd.filter) == 0 && cd.defaultFilter != "" {
		cond, err := NewCondition(cd.defaultFilter)
		if err != nil {
			return err
		}
		cd.filter = append(cd.filter, cond)
	}

	if len(cd.warnThreshold) == 0 && cd.defaultWarning != "" {
		cond, err := NewCondition(cd.defaultWarning)
		if err != nil {
			return err
		}
		cd.warnThreshold = append(cd.warnThreshold, cond)
	}

	if len(cd.critThreshold) == 0 && cd.defaultCritical != "" {
		cond, err := NewCondition(cd.defaultCritical)
		if err != nil {
			return err
		}
		cd.critThreshold = append(cd.critThreshold, cond)
	}

	return nil
}

func (cd *CheckData) applyConditionAlias() {
	if len(cd.conditionAlias) == 0 {
		return
	}
	cd.applyConditionAliasList(cd.filter)
	cd.applyConditionAliasList(cd.warnThreshold)
	cd.applyConditionAliasList(cd.critThreshold)
	cd.applyConditionAliasList(cd.okThreshold)
}

func (cd *CheckData) applyConditionAliasList(cond []*Condition) {
	for _, cond := range cond {
		if len(cond.group) > 0 {
			cd.applyConditionAliasList(cond.group)

			return
		}

		for replaceKeyword, aliasMap := range cd.conditionAlias {
			if cond.keyword == replaceKeyword {
				valStr := fmt.Sprintf("%v", cond.value)
				if repl, ok := aliasMap[valStr]; ok {
					cond.value = repl
				}
			}
		}
	}
}
