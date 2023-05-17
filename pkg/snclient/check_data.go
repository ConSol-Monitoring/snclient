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
	filter          []*Condition
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
		switch l["state"] {
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
	switch macros["state"] {
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
	case macros["warn_count"] != "0":
		cd.result.EscalateStatus(1)
	}
}

// Check conditions against given data and set result state
func (cd *CheckData) Check(data map[string]string, warnCond, critCond, okCond []*Condition) {
	data["state"] = fmt.Sprintf("%d", CheckExitOK)

	for i := range warnCond {
		if warnCond[i].Match(data) {
			data["state"] = fmt.Sprintf("%d", CheckExitWarning)
		}
	}

	for i := range critCond {
		if critCond[i].Match(data) {
			data["state"] = fmt.Sprintf("%d", CheckExitCritical)
		}
	}

	for i := range okCond {
		if okCond[i].Match(data) {
			data["state"] = fmt.Sprintf("%d", CheckExitOK)
		}
	}
}

// MatchFilter returns true if {name: value} matches any filter
func (cd *CheckData) MatchFilter(name, value string) bool {
	data := map[string]string{name: value}
	for _, cond := range cd.filter {
		if cond.Match(data) {
			return true
		}
	}

	return false
}

// Filter data map by conditions and return filtered list
func (cd *CheckData) Filter(conditions []*Condition, data []map[string]string) []map[string]string {
	result := make([]map[string]string, 0)

	for num := range data {
		matched := false
		for i := range conditions {
			if conditions[i].Match(data[num]) {
				break
			}
		}
		if matched {
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
	for _, v := range args {
		split := strings.SplitN(v, "=", 2)
		if len(split) == 1 {
			split = append(split, "")
		}
		switch split[0] {
		case "ok":
			cond, err := NewCondition(split[1])
			if err != nil {
				return nil, err
			}
			cd.okThreshold = append(cd.okThreshold, cond)
		case "warn", "warning":
			cond, err := NewCondition(split[1])
			if err != nil {
				return nil, err
			}
			cd.warnThreshold = append(cd.warnThreshold, cond)
		case "crit", "critical":
			cond, err := NewCondition(split[1])
			if err != nil {
				return nil, err
			}
			cd.critThreshold = append(cd.critThreshold, cond)
		case "debug":
			cd.debug = split[1]
			if cd.debug == "" {
				cd.debug = "debug"
			}
		case "detail-syntax":
			cd.detailSyntax = split[1]
		case "top-syntax":
			cd.topSyntax = split[1]
		case "ok-syntax":
			cd.okSyntax = split[1]
		case "empty-syntax":
			cd.emptySyntax = split[1]
		case "empty-state":
			cd.emptyState = cd.parseStateString(split[1])
		case "filter":
			cond, err := NewCondition(split[1])
			if err != nil {
				return nil, err
			}
			cd.filter = append(cd.filter, cond)
		default:
			argList = append(argList, Argument{key: split[0], value: split[1]})
		}
	}

	err := cd.setFallbacks()
	if err != nil {
		return nil, err
	}

	raiseLogLevel(cd.debug)

	return argList, nil
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
