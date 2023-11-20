package snclient

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"pkg/convert"
	"pkg/humanize"
	"pkg/utils"

	"golang.org/x/exp/slices"
)

var (
	// Variable to use in Threshold Min/Max
	Zero    = float64(0)
	Hundred = float64(100)

	DefaultCheckTimeout = float64(60)
)

// InventoryMode sets available inventory move
type InventoryMode uint8

const (
	NoInventory      InventoryMode = iota
	ListInventory                  // calls this check and uses listDetails
	ScriptsInventory               // does not call this check and puts it into the scripts section
)

type CommaStringList []string

type CheckArgument struct {
	value       interface{} // reference to storage pointer
	description string      // used in help
	isFilter    bool        // if true, default filter is not used when this argument is set
}

// Implemented defines the available supported operating systems
type Implemented uint8

// Implemented defines the available supported operating systems
const (
	_ Implemented = iota
	Windows
	Linux
	Darwin
	FreeBSD

	ALL = Windows | Linux | Darwin | FreeBSD
)

// ShowHelp defines available help options
type ShowHelp uint8

// ShowHelp defines available help options
const (
	_ ShowHelp = iota
	Markdown
	PluginHelp
)

type CheckAttribute struct {
	name        string
	description string
}

// CheckData contains the runtime data of a generic check plugin
type CheckData struct {
	noCopy          noCopy
	name            string
	description     string
	debug           string
	defaultFilter   string
	conditionAlias  map[string]map[string]string // replacement map of equivalent condition values
	args            map[string]CheckArgument
	extraArgs       map[string]CheckArgument // internal, map of expanded args
	argsPassthrough bool                     // allow arbitrary arguments without complaining about unknown argument
	rawArgs         []string
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
	listCombine     string // join string for detail list
	showAll         bool
	result          *CheckResult
	showHelp        ShowHelp
	timeout         float64
	perfConfig      []PerfConfig
	hasInventory    InventoryMode
	output          string
	implemented     Implemented
	attributes      []CheckAttribute
	exampleDefault  string
	exampleArgs     string
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

	cd.result.Raw = cd
	if cd.output == "inventory_json" {
		return cd.result, nil
	}

	return cd.finalizeOutput()
}

func (cd *CheckData) finalizeOutput() (*CheckResult, error) {
	if len(cd.listData) > 0 {
		log.Tracef("list data:")
		for _, entry := range cd.listData {
			if skipped, ok := entry["_skip"]; ok {
				if skipped == "1" {
					continue
				}
			}
			// not yet filtered errors are fatal
			if errMsg, ok := entry["_error"]; ok {
				return nil, fmt.Errorf("%s", errMsg)
			}
			cd.Check(entry, cd.warnThreshold, cd.critThreshold, cd.okThreshold)
			log.Tracef(" - %v", entry)
		}
	}

	var finalMacros map[string]string
	if len(cd.listData) == 1 {
		finalMacros = cd.buildListMacrosFromSingleEntry()
	} else {
		finalMacros = cd.buildListMacros()
	}

	err := cd.result.ApplyPerfConfig(cd.perfConfig)
	if err != nil {
		return nil, fmt.Errorf("%s", err.Error())
	}

	cd.Check(finalMacros, cd.warnThreshold, cd.critThreshold, cd.okThreshold)
	cd.setStateFromMaps(finalMacros)

	switch {
	case cd.result.Output != "":
		// already set, leave it
		return cd.result, nil
	case len(cd.filter) > 0 && len(cd.listData) == 0:
		cd.result.Output = cd.emptySyntax
		cd.result.State = cd.emptyState
	case cd.showAll:
		cd.result.Output = "%(status): %(list)"
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

	if cd.listCombine == "" {
		cd.listCombine = ", "
	}
	result := map[string]string{
		"count":         fmt.Sprintf("%d", len(list)),
		"list":          strings.Join(list, cd.listCombine),
		"ok_count":      fmt.Sprintf("%d", len(okList)),
		"ok_list":       "",
		"warn_count":    fmt.Sprintf("%d", len(warnList)),
		"warn_list":     "",
		"crit_count":    fmt.Sprintf("%d", len(critList)),
		"crit_list":     "",
		"problem_count": fmt.Sprintf("%d", len(warnList)+len(critList)),
		"problem_list":  "",
		"detail_list":   "",
	}

	problemList := []string{}
	detailList := []string{}

	// if there is only one problem, there is no need to add brackets
	if len(critList) > 0 {
		result["crit_list"] = "critical(" + strings.Join(critList, ", ") + ")"
		problemList = append(problemList, result["crit_list"])
		detailList = append(detailList, result["crit_list"])
	}
	if len(warnList) > 0 {
		result["warn_list"] = "warning(" + strings.Join(warnList, ", ") + ")"
		problemList = append(problemList, result["warn_list"])
		detailList = append(detailList, result["warn_list"])
	}
	if len(okList) > 0 {
		result["ok_list"] = strings.Join(okList, ", ")
		detailList = append(detailList, result["ok_list"])
	}

	result["problem_list"] = strings.Join(problemList, " ")
	result["detail_list"] = strings.Join(detailList, " ")

	return result
}

func (cd *CheckData) buildListMacrosFromSingleEntry() map[string]string {
	entry := cd.listData[0]
	expanded := ReplaceMacros(cd.detailSyntax, entry)

	result := map[string]string{
		"count":         "1",
		"list":          expanded,
		"ok_count":      "0",
		"ok_list":       "",
		"warn_count":    "0",
		"warn_list":     "",
		"crit_count":    "0",
		"crit_list":     "",
		"problem_count": "0",
		"problem_list":  "",
		"detail_list":   expanded,
	}

	switch entry["_state"] {
	case "0":
		result["ok_list"] = expanded
		result["ok_count"] = "1"
	case "1":
		result["problem_list"] = expanded
		result["warn_list"] = expanded
		result["warn_count"] = "1"
	case "2":
		result["problem_list"] = expanded
		result["crit_list"] = expanded
		result["crit_count"] = "1"
	}

	return result
}

// setStateFromMaps sets main state from _state or list counts.
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

// Check tries warn/crit/ok conditions against given data and sets result state.
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

	return cd.MatchFilterMap(data)
}

// MatchFilterMap returns true if given map matches any filter
func (cd *CheckData) MatchFilterMap(data map[string]string) bool {
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

// MatchMapCondition returns true if listEntry matches filter
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

// Filter data map by conditions and return filtered list.
// ALl items not matching given filter will be removed.
func (cd *CheckData) Filter(conditions []*Condition, data []map[string]string) []map[string]string {
	if len(conditions) == 0 {
		return data
	}
	result := make([]map[string]string, 0)

	for num := range data {
		if cd.MatchMapCondition(conditions, data[num]) {
			result = append(result, data[num])
		}
	}

	return result
}

// parseStateString translates string naemon state to int64
func (cd *CheckData) parseStateString(state string) int64 {
	switch strings.ToLower(state) {
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
	cd.rawArgs = args
	appendArgs := map[string]bool{}
	argList := make([]Argument, 0, len(args))
	applyDefaultFilter := true
	cd.expandArgDefinitions()
	numArgs := len(args)
	for idx := 0; idx < numArgs; idx++ {
		argExpr := cd.removeQuotes(args[idx])
		split := strings.SplitN(argExpr, "=", 2)
		keyword := cd.removeQuotes(split[0])
		argValue, newIdx, err := cd.fetchNextArg(args, split, keyword, idx, numArgs)
		if err != nil {
			return nil, err
		}
		idx = newIdx
		argValue = cd.removeQuotes(argValue)
		if a, ok := cd.args[keyword]; ok && a.isFilter {
			applyDefaultFilter = false
		}
		if a, ok := cd.extraArgs[keyword]; ok && a.isFilter {
			applyDefaultFilter = false
		}
		switch keyword {
		case "help":
			switch argValue {
			case "markdown", "md":
				cd.showHelp = Markdown
			default:
				cd.showHelp = PluginHelp
			}

			return nil, nil
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
		case "show-all":
			if argValue == "" {
				cd.showAll = true
			} else {
				showAll, err := convert.BoolE(argValue)
				if err != nil {
					return nil, fmt.Errorf("parseBool %s: %s", argValue, err.Error())
				}
				cd.showAll = showAll
			}
		case "filter":
			applyDefaultFilter = false
			cond, err := NewCondition(argValue)
			if err != nil {
				return nil, err
			}
			cd.filter = append(cd.filter, cond)
		case "timeout":
			timeout, err := convert.Float64E(argValue)
			if err != nil {
				return nil, fmt.Errorf("timeout parse error: %s", err.Error())
			}
			cd.timeout = timeout
		case "perf-config":
			perf, err := NewPerfConfig(argValue)
			if err != nil {
				return nil, err
			}
			cd.perfConfig = append(cd.perfConfig, perf...)
		case "output":
			cd.output = argValue
		default:
			parsed, err := cd.parseAnyArg(appendArgs, argExpr, keyword, argValue)
			switch {
			case err != nil:
				return nil, err
			case parsed:
				// ok
			case !cd.argsPassthrough:
				return nil, fmt.Errorf("unknown argument: %s", keyword)
			default:
				argList = append(argList, Argument{key: keyword, value: argValue})
			}
		}
	}

	err := cd.setFallbacks(applyDefaultFilter)
	if err != nil {
		return nil, err
	}

	cd.applyConditionAlias()

	// increase logLevel temporary if debug arg is set
	if cd.debug != "" {
		raiseLogLevel(cd.debug)
	}

	return argList, nil
}

func (cd *CheckData) fetchNextArg(args, split []string, keyword string, idx, numArgs int) (argVal string, newIdx int, err error) {
	if len(split) == 2 {
		return split[1], idx, nil
	}
	arg, ok := cd.args[keyword]
	if !ok {
		arg, ok = cd.extraArgs[keyword]
		if !ok {
			return "", idx, nil
		}
	}

	_, ok = arg.value.(*bool)
	if ok {
		return "", idx, nil
	}

	// known arg and not a bool value -> consume next value
	idx++
	if idx >= numArgs {
		return "", idx, fmt.Errorf("argument value expected for %s", keyword)
	}

	return args[idx], idx, nil
}

// parseAnyArg parses args into the args map with custom arguments
func (cd *CheckData) parseAnyArg(appendArgs map[string]bool, argExpr, keyword, argValue string) (bool, error) {
	arg, ok := cd.args[keyword]
	if !ok {
		arg, ok = cd.extraArgs[keyword]
		if !ok {
			return false, nil
		}
	}

	switch argRef := arg.value.(type) {
	case *[]string:
		if _, ok := appendArgs[keyword]; !ok {
			// first time this arg occurs, empty default lists
			empty := make([]string, 0)
			*argRef = empty
		}
		*argRef = append(*argRef, argValue)
		appendArgs[keyword] = true
	case *CommaStringList:
		if _, ok := appendArgs[keyword]; !ok {
			// first time this arg occurs, empty default lists
			empty := make([]string, 0)
			*argRef = empty
		}
		*argRef = append(*argRef, strings.Split(argValue, ",")...)
		appendArgs[keyword] = true
	case *string:
		*argRef = argValue
	case *float64:
		f, err := strconv.ParseFloat(argValue, 64)
		if err != nil {
			return true, fmt.Errorf("parseFloat %s: %s", argExpr, err.Error())
		}
		*argRef = f
	case *int64:
		i, err := strconv.ParseInt(argValue, 10, 64)
		if err != nil {
			return true, fmt.Errorf("parseInt %s: %s", argExpr, err.Error())
		}
		*argRef = i
	case *bool:
		if argValue == "" {
			b := true
			*argRef = b
		} else {
			b, err := convert.BoolE(argValue)
			if err != nil {
				return true, fmt.Errorf("parseBool %s: %s", argValue, err.Error())
			}
			*argRef = b
		}
	default:
		log.Errorf("unsupported args type: %T in %s", argRef, argExpr)
	}

	return true, nil
}

// removeQuotes remove single/double quotes around string
func (cd *CheckData) removeQuotes(str string) string {
	str = strings.TrimSpace(str)
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

// setFallbacks sets default filter/warn/crit thresholds unless already set.
func (cd *CheckData) setFallbacks(applyDefaultFilter bool) error {
	if applyDefaultFilter && cd.defaultFilter != "" {
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

	if cd.timeout == 0 {
		cd.timeout = DefaultCheckTimeout
	}

	return nil
}

// apply condition aliases to all filter/warn/crit/ok conditions.
func (cd *CheckData) applyConditionAlias() {
	if len(cd.conditionAlias) == 0 {
		return
	}
	cd.applyConditionAliasList(cd.filter)
	cd.applyConditionAliasList(cd.warnThreshold)
	cd.applyConditionAliasList(cd.critThreshold)
	cd.applyConditionAliasList(cd.okThreshold)
}

// apply condition aliases to given conditions.
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

// HasThreshold returns true is the warn/crit threshold use at least one condition with the given name.
func (cd *CheckData) HasThreshold(name string) bool {
	if cd.hasThresholdCond(cd.warnThreshold, name) {
		return true
	}
	if cd.hasThresholdCond(cd.critThreshold, name) {
		return true
	}

	return false
}

// hasThresholdCond returns true is the given list of conditions uses the given name at least once.
func (cd *CheckData) hasThresholdCond(condList []*Condition, name string) bool {
	for _, cond := range condList {
		if len(cond.group) > 0 {
			return cd.hasThresholdCond(cond.group, name)
		}

		if cond.keyword == name {
			return true
		}
	}

	return false
}

// SetDefaultThresholdUnit sets default unit for all threshold conditions matching
// the name and not having a unit already
func (cd *CheckData) SetDefaultThresholdUnit(defaultUnit string, names []string) {
	setDefault := func(cond *Condition) bool {
		if len(cond.group) == 0 && cond.unit == "" {
			for _, name := range names {
				if name == cond.keyword {
					cond.unit = defaultUnit
				}
			}
		}

		return true
	}
	cd.VisitAll(cd.warnThreshold, setDefault)
	cd.VisitAll(cd.critThreshold, setDefault)
	cd.VisitAll(cd.okThreshold, setDefault)
	cd.VisitAll(cd.filter, setDefault)
}

// ExpandThresholdUnit multiplies the threshold value if the unit matches the exponents. Unit is then replaced with the targetUnit.
func (cd *CheckData) ExpandThresholdUnit(exponents []string, targetUnit string, names []string) {
	apply := func(cond *Condition) bool {
		if len(cond.group) > 0 {
			return true
		}
		unit := strings.ToLower(cond.unit)
		if slices.Contains(names, cond.keyword) && slices.Contains(exponents, unit) {
			val, err := humanize.ParseBytes(fmt.Sprintf("%f%s%s", convert.Float64(cond.value), cond.unit, targetUnit))
			if err == nil {
				cond.unit = targetUnit
				cond.value = val
			}
		}

		return true
	}
	cd.VisitAll(cd.warnThreshold, apply)
	cd.VisitAll(cd.critThreshold, apply)
	cd.VisitAll(cd.okThreshold, apply)
}

// VisitAll calls callback recursively for each condition until callback returns false
func (cd *CheckData) VisitAll(condList []*Condition, callback func(*Condition) bool) bool {
	for _, cond := range condList {
		if len(cond.group) > 0 {
			if !cd.VisitAll(cond.group, callback) {
				return false
			}
		} else {
			if !callback(cond) {
				return false
			}
		}
	}

	return true
}

func (cd *CheckData) CloneThreshold(srcThreshold []*Condition) (cloned []*Condition) {
	cloned = make([]*Condition, 0)

	for i := range srcThreshold {
		cloned = append(cloned, srcThreshold[i].Clone())
	}

	return cloned
}

func (cd *CheckData) TransformThreshold(srcThreshold []*Condition, srcName, targetName, srcUnit, targetUnit string, total float64) (threshold []*Condition) {
	transformed := cd.CloneThreshold(srcThreshold)
	// Warning:  check.TransformThreshold(check.warnThreshold, "used", name, "%", "B", total),
	applyChange := func(cond *Condition) bool {
		if cond.keyword != srcName {
			return true
		}
		cond.keyword = targetName
		if cond.unit != srcUnit {
			return true
		}

		switch {
		case srcUnit == "%":
			pct := convert.Float64(cond.value)
			val := pct / 100 * total
			switch {
			case strings.EqualFold(targetUnit, "b"):
				cond.value = math.Round(val)
			default:
				cond.value = utils.ToPrecision(val, 3)
			}
			cond.unit = targetUnit
		case targetUnit == "%":
			val := convert.Float64(cond.value)
			pct := (val * 100) / total
			cond.value = utils.ToPrecision(pct, 2)
			cond.unit = targetUnit
		default:
			log.Errorf("unsupported src unit in threshold transition: %s", srcUnit)
		}

		return true
	}
	cd.VisitAll(transformed, applyChange)

	return transformed
}

// replaces source keywords in threshold with new keyword
func (cd *CheckData) TransformMultipleKeywords(srcKeywords []string, targetKeyword string, srcThreshold []*Condition) (threshold []*Condition) {
	transformed := cd.CloneThreshold(srcThreshold)
	applyChange := func(cond *Condition) bool {
		if !slices.Contains(srcKeywords, cond.keyword) {
			return true
		}
		cond.keyword = targetKeyword

		return true
	}
	cd.VisitAll(transformed, applyChange)

	return transformed
}

func (cd *CheckData) AddBytePercentMetrics(threshold, perfLabel string, val, total float64) {
	percent := float64(0)
	if threshold == "used" {
		percent = 100
	}
	if total > 0 {
		percent = val * 100 / total
	}
	pctName := perfLabel + " %"
	cd.result.Metrics = append(cd.result.Metrics,
		&CheckMetric{
			Name:     perfLabel,
			Unit:     "B",
			Value:    int64(val),
			Warning:  cd.TransformThreshold(cd.warnThreshold, threshold, perfLabel, "%", "B", total),
			Critical: cd.TransformThreshold(cd.critThreshold, threshold, perfLabel, "%", "B", total),
			Min:      &Zero,
			Max:      &total,
		},
		&CheckMetric{
			Name:     pctName,
			Unit:     "%",
			Value:    utils.ToPrecision(percent, 1),
			Warning:  cd.TransformThreshold(cd.warnThreshold, threshold, pctName, "B", "%", total),
			Critical: cd.TransformThreshold(cd.critThreshold, threshold, pctName, "B", "%", total),
			Min:      &Zero,
			Max:      &Hundred,
		},
	)
}

func (cd *CheckData) AddPercentMetrics(threshold, perfLabel string, val, total float64) {
	percent := float64(0)
	if strings.Contains(threshold, "used") {
		percent = 100
	} else if strings.Contains(threshold, "free") {
		percent = 0
	}
	if total > 0 {
		percent = val * 100 / total
	}
	cd.result.Metrics = append(cd.result.Metrics,
		&CheckMetric{
			Name:          perfLabel,
			ThresholdName: threshold,
			Unit:          "%",
			Value:         utils.ToPrecision(percent, 1),
			Warning:       cd.warnThreshold,
			Critical:      cd.critThreshold,
			Min:           &Zero,
			Max:           &Hundred,
		},
	)
}

// expand arg definitions separated by pipe symbol
// ex.: -w|--warning
func (cd *CheckData) expandArgDefinitions() {
	cd.extraArgs = make(map[string]CheckArgument)
	for k, arg := range cd.args {
		keys := strings.Split(k, "|")
		for _, key := range keys {
			key = strings.TrimSpace(key)
			cd.extraArgs[key] = arg
		}
	}
}

// generate help, set docs to true to generate markdown docs page, otherwise check plugin help page will be generated.
func (cd *CheckData) Help(format ShowHelp) string {
	out := cd.helpHeader(format)
	out += cd.helpDefaultArguments(format)
	out += cd.helpSpecificArguments(format)
	out += cd.helpAttributes(format)

	out = strings.TrimSpace(out)

	return out
}

func (cd *CheckData) helpHeader(format ShowHelp) string {
	out := ""
	if format == Markdown {
		out += cd.helpHeaderMarkdown(format)
	} else {
		out += fmt.Sprintf("Usage: %s [<options>] [<filter>]\n\n", cd.name)
		out += fmt.Sprintf("    %s\n\n", cd.description)
	}

	return out
}

func (cd *CheckData) helpHeaderMarkdown(format ShowHelp) string {
	out := ""
	out += fmt.Sprintf("---\ntitle: %s\n---\n\n", strings.TrimPrefix(cd.name, "check_"))
	out += fmt.Sprintf("## %s\n\n", cd.name)
	out += fmt.Sprintf("%s\n\n", cd.description)
	out += "- [Examples](#examples)\n"
	out += "- [Argument Defaults](#argument-defaults)\n"
	if cd.attributes != nil && len(cd.attributes) > 0 {
		out += "- [Attributes](#attributes)\n"
	}
	out += "\n"

	if cd.implemented > 0 {
		out += cd.helpImplemented(format)
	}

	out += "## Examples\n\n"
	if cd.exampleDefault != "" {
		out += "### Default Check\n\n"
		out += fmt.Sprintf("    %s\n\n", strings.TrimSpace(cd.exampleDefault))
	}
	out += "### Example using NRPE and Naemon\n\n"
	out += fmt.Sprintf(`Naemon Config

    define command{
        command_name    check_nrpe
        command_line    $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name               testhost
        service_description     %s
        use                     generic-service
        check_command           check_nrpe!%s!%s
    }`, cd.name, cd.name, cd.exampleArgs)
	out += "\n\n"

	return out
}

func (cd *CheckData) helpImplemented(format ShowHelp) string {
	out := ""
	type implTableData struct {
		windows string
		linux   string
		freebsd string
		osx     string
	}
	header := []utils.ASCIITableHeader{
		{Name: "Windows", Field: "windows", Centered: true},
		{Name: "Linux", Field: "linux", Centered: true},
		{Name: "FreeBSD", Field: "freebsd", Centered: true},
		{Name: "MacOSX", Field: "osx", Centered: true},
	}
	implemented := implTableData{}
	if cd.implemented&Windows > 0 {
		implemented.windows = ":white_check_mark:"
	}
	if cd.implemented&Linux > 0 {
		implemented.linux = ":white_check_mark:"
	}
	if cd.implemented&FreeBSD > 0 {
		implemented.freebsd = ":white_check_mark:"
	}
	if cd.implemented&Darwin > 0 {
		implemented.osx = ":white_check_mark:"
	}
	table, err := utils.ASCIITable(header, []implTableData{implemented}, format == Markdown)
	if err != nil {
		log.Errorf("ascii table failed: %s", err.Error())
	}
	out += "## Implementation\n\n"
	out += table
	out += "\n"

	return out
}

func (cd *CheckData) helpDefaultArguments(format ShowHelp) string {
	out := ""
	if format == Markdown {
		out += "## Argument Defaults\n\n"
	} else {
		out += "Argument Defaults:\n\n"
	}

	type defaultArg struct {
		name     string
		defaults string
	}
	defaultArgs := []defaultArg{}

	if cd.defaultFilter != "" {
		defaultArgs = append(defaultArgs, defaultArg{name: "filter", defaults: cd.defaultFilter})
	}
	if cd.defaultWarning != "" {
		defaultArgs = append(defaultArgs, defaultArg{name: "warning", defaults: cd.defaultWarning})
	}
	if cd.defaultCritical != "" {
		defaultArgs = append(defaultArgs, defaultArg{name: "critcal", defaults: cd.defaultCritical})
	}
	defaultArgs = append(defaultArgs,
		defaultArg{name: "empty-state", defaults: fmt.Sprintf("%d (%s)", cd.emptyState, convert.StateString(cd.emptyState))},
		defaultArg{name: "empty-syntax", defaults: cd.emptySyntax},
		defaultArg{name: "top-syntax", defaults: cd.topSyntax},
		defaultArg{name: "ok-syntax", defaults: cd.okSyntax},
		defaultArg{name: "detail-syntax", defaults: cd.detailSyntax},
	)

	header := []utils.ASCIITableHeader{
		{Name: "Argument", Field: "name"},
		{Name: "Default Value", Field: "defaults"},
	}
	table, err := utils.ASCIITable(header, defaultArgs, format == Markdown)
	if err != nil {
		log.Errorf("ascii table failed: %s", err.Error())
	}
	if format == Markdown {
		out += table
	} else {
		out += "    " + strings.TrimSpace(strings.Join(strings.Split(table, "\n"), "\n    ")) + "\n\n"
	}
	out += "\n"

	return out
}

func (cd *CheckData) helpSpecificArguments(format ShowHelp) string {
	out := ""

	if format == Markdown {
		out += "## Check Specific Arguments\n\n"
	} else {
		out += "Check Specific Arguments:\n\n"
	}
	if cd.args == nil || len(cd.args) == 0 {
		if format == Markdown {
			out += "None\n\n"
		} else {
			out += "    None\n\n"
		}
		return out
	}

	keys := []string{}
	for k := range cd.args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	type attr struct {
		name        string
		description string
	}
	attributes := []attr{}
	for _, k := range keys {
		a := cd.args[k]
		attributes = append(attributes, attr{name: k, description: a.description})
	}

	header := []utils.ASCIITableHeader{
		{Name: "Argument", Field: "name"},
		{Name: "Description", Field: "description"},
	}
	table, err := utils.ASCIITable(header, attributes, format == Markdown)
	if err != nil {
		log.Errorf("ascii table failed: %s", err.Error())
	}
	if format == Markdown {
		out += table
	} else {
		out += "    " + strings.TrimSpace(strings.Join(strings.Split(table, "\n"), "\n    ")) + "\n\n"
	}
	out += "\n"

	return out
}

func (cd *CheckData) helpAttributes(format ShowHelp) string {
	out := ""
	if cd.attributes == nil || len(cd.attributes) == 0 {
		return out
	}

	if format == Markdown {
		out += "## Attributes\n\n"
		out += "### Check Specific Attributes\n\n"
	} else {
		out += "Check Specific Attributes:\n\n    "
	}
	out += "these can be used in filters and thresholds (along with the default attributes):\n\n"

	header := []utils.ASCIITableHeader{
		{Name: "Attribute", Field: "name"},
		{Name: "Description", Field: "description"},
	}
	table, err := utils.ASCIITable(header, cd.attributes, format == Markdown)
	if err != nil {
		log.Errorf("ascii table failed: %s", err.Error())
	}
	if format == Markdown {
		out += table
	} else {
		out += "    " + strings.TrimSpace(strings.Join(strings.Split(table, "\n"), "\n    ")) + "\n\n"
	}
	out += "\n"

	return out
}
