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
	log.Tracef("details: %v", cd.details)

	cd.Check(CheckExitCritical, cd.critThreshold, cd.details)
	cd.Check(CheckExitWarning, cd.warnThreshold, cd.details)

	if len(cd.listData) > 0 {
		log.Tracef("list data:")
		for _, l := range cd.listData {
			log.Tracef(" - %v", l)
			cd.Check(CheckExitCritical, cd.critThreshold, l)
			cd.Check(CheckExitWarning, cd.warnThreshold, l)
		}
	}

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

	list := []string{}
	for _, l := range cd.listData {
		el := ReplaceMacros(cd.detailSyntax, l)
		list = append(list, el)
	}

	finalMacros := map[string]string{
		"count": fmt.Sprintf("%d", len(list)),
		"list":  strings.Join(list, ", "),
	}
	cd.result.Finalize(cd.details, finalMacros)

	return cd.result, nil
}

// Check conditions against given data and set result state
func (cd *CheckData) Check(state int64, conditions []*Condition, data map[string]string) {
	// no need to escalate state anymore
	if cd.result.State >= state {
		return
	}

	for i := range conditions {
		if conditions[i].Match(data) {
			cd.result.State = state

			return
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
