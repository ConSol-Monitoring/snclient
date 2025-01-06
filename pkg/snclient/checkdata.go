package snclient

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/humanize"
	"github.com/consol-monitoring/snclient/pkg/utils"
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
	NoCallInventory                // does not call this check and puts single entry into inventory
)

type CommaStringList []string

type CheckArgument struct {
	value           interface{} // reference to storage pointer
	description     string      // used in help
	isFilter        bool        // if true, default filter is not used when this argument is set
	defaultCritical string      // overrides default filter if argument is used
	defaultWarning  string      // same for critical condition
}

// Implemented defines the available supported operating systems
type Implemented uint8

// Implemented defines the available supported operating systems
const (
	_ Implemented = 1 << iota
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
	noCopy                 noCopy
	name                   string
	description            string
	docTitle               string
	usage                  string
	defaultFilter          string
	conditionAlias         map[string]map[string]string // replacement map of equivalent condition values
	args                   map[string]CheckArgument
	extraArgs              map[string]CheckArgument // internal, map of expanded args
	argsPassthrough        bool                     // allow arbitrary arguments without complaining about unknown argument
	hasArgsSupplied        map[string]bool          // map which is true if a arg has been specified on the command line
	rawArgs                []string
	filter                 ConditionList // if set, only show entries matching this filter set
	warnThreshold          ConditionList
	defaultWarning         string
	critThreshold          ConditionList
	defaultCritical        string
	okThreshold            ConditionList
	detailSyntax           string
	topSyntax              string
	okSyntax               string
	hasArgsFilter          bool // will be true if any arg supplied which has isFilter set
	emptySyntax            string
	emptyState             int64
	details                map[string]string
	listData               []map[string]string
	listCombine            string // join string for detail list
	showAll                bool
	addCountMetrics        bool
	addProblemCountMetrics bool
	result                 *CheckResult
	showHelp               ShowHelp
	timeout                float64
	perfConfig             []PerfConfig
	perfSyntax             string
	hasInventory           InventoryMode
	output                 string
	implemented            Implemented
	attributes             []CheckAttribute
	exampleDefault         string
	exampleArgs            string
}

func (cd *CheckData) Finalize() (*CheckResult, error) {
	defer restoreLogLevel()
	log.Tracef("finalize check results:")
	if cd.details == nil {
		cd.details = map[string]string{}
	}
	cd.details["top-syntax"] = cd.topSyntax
	cd.details["ok-syntax"] = cd.okSyntax
	cd.details["empty-syntax"] = cd.emptySyntax
	cd.details["detail-syntax"] = cd.detailSyntax
	log.Debugf("filter:             %s", cd.filter.String())
	log.Debugf("condition  warning: %s", cd.warnThreshold.String())
	log.Debugf("condition critical: %s", cd.critThreshold.String())
	log.Debugf("condition       ok: %s", cd.okThreshold.String())
	cd.Check(cd.details, cd.warnThreshold, cd.critThreshold, cd.okThreshold)
	log.Tracef("details: %#v", cd.details)

	// apply final filter
	cd.listData = cd.Filter(cd.filter, cd.listData)

	if cd.result == nil {
		cd.result = &CheckResult{}
	}
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
			if entry["_skip"] == "1" {
				continue
			}

			// not yet filtered errors are fatal
			errMsg := entry["_error"]
			exitCode := entry["_exit"]
			if exitCode != "" {
				cd.result.State = convert.Int64(exitCode)
				cd.result.Output = fmt.Sprintf("%s - %s", convert.StateString(cd.result.State), errMsg)
				log.Tracef(" - %#v", entry)

				return cd.result, nil
			}

			if errMsg != "" {
				log.Tracef(" - %#v", entry)

				return nil, fmt.Errorf("%s", errMsg)
			}
			cd.Check(entry, cd.warnThreshold, cd.critThreshold, cd.okThreshold)
			log.Tracef(" - %#v", entry)
		}
	}

	var finalMacros map[string]string
	log.Tracef("detail template: %s", cd.detailSyntax)
	if len(cd.listData) == 1 {
		finalMacros = cd.buildListMacrosFromSingleEntry()
	} else {
		finalMacros = cd.buildListMacros()
	}
	err := cd.result.ApplyPerfConfig(cd.perfConfig)
	if err != nil {
		return nil, fmt.Errorf("%s", err.Error())
	}

	cd.result.ApplyPerfSyntax(cd.perfSyntax)

	cd.Check(finalMacros, cd.warnThreshold, cd.critThreshold, cd.okThreshold)
	cd.setStateFromMaps(finalMacros)
	cd.CheckMetrics(cd.warnThreshold, cd.critThreshold, cd.okThreshold)

	switch {
	case cd.result.Output != "":
		// already set, leave it
		return cd.result, nil
	case len(cd.listData) == 0 && (len(cd.filter) > 0 || cd.hasArgsFilter):
		if cd.emptySyntax != "" {
			cd.result.Output = cd.emptySyntax
		}
		if !cd.HasThreshold("count") {
			cd.result.State = cd.emptyState
		}
	case cd.showAll:
		cd.result.Output = "%(status) - %(list)"
	case cd.result.State == 0 && cd.okSyntax != "":
		cd.result.Output = cd.okSyntax
	default:
		cd.result.Output = cd.topSyntax
	}

	log.Tracef("output template: %s", cd.result.Output)

	cd.result.Finalize(cd.details, finalMacros)

	return cd.result, nil
}

func (cd *CheckData) buildListMacros() map[string]string {
	list := []string{}
	okList := make([]string, 0)
	warnList := make([]string, 0)
	critList := make([]string, 0)
	count := int64(0)
	okCount := int64(0)
	warnCount := int64(0)
	critCount := int64(0)
	for _, entry := range cd.listData {
		weight := int64(1)
		if w, ok := entry["_count"]; ok {
			weight = convert.Int64(w)
		}
		count += weight
		if _, ok := entry["count"]; !ok {
			entry["count"] = fmt.Sprintf("%d", weight)
		}
		expanded, err := ReplaceTemplate(cd.detailSyntax, entry)
		if err != nil {
			log.Debugf("replacing syntax failed %s: %s", cd.detailSyntax, err.Error())
		}
		list = append(list, expanded)
		switch entry["_state"] {
		case "0":
			okList = append(okList, expanded)
			okCount += weight
		case "1":
			warnList = append(warnList, expanded)
			warnCount += weight
		case "2":
			critList = append(critList, expanded)
			critCount += weight
		}
	}

	if cd.listCombine == "" {
		cd.listCombine = ", "
	}
	result := map[string]string{
		"count":         fmt.Sprintf("%d", count),
		"list":          strings.Join(list, cd.listCombine),
		"ok_count":      fmt.Sprintf("%d", okCount),
		"ok_list":       "",
		"warn_count":    fmt.Sprintf("%d", warnCount),
		"warn_list":     "",
		"crit_count":    fmt.Sprintf("%d", critCount),
		"crit_list":     "",
		"problem_count": fmt.Sprintf("%d", warnCount+critCount),
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

	cd.buildCountMetrics(len(list), len(critList), len(warnList))

	return result
}

func (cd *CheckData) buildListMacrosFromSingleEntry() map[string]string {
	entry := cd.listData[0]
	expanded, err := ReplaceTemplate(cd.detailSyntax, entry)
	if err != nil {
		log.Debugf("replacing template failed: %s: %s", cd.detailSyntax, err.Error())
	}

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

	numWarn := 0
	numCrit := 0
	switch entry["_state"] {
	case "0":
		result["ok_list"] = expanded
		result["ok_count"] = "1"
	case "1":
		result["problem_list"] = expanded
		result["warn_list"] = expanded
		result["warn_count"] = "1"
		numWarn = 1
	case "2":
		result["problem_list"] = expanded
		result["crit_list"] = expanded
		result["crit_count"] = "1"
		numCrit = 1
	}

	cd.buildCountMetrics(1, numCrit, numWarn)

	return result
}

func (cd *CheckData) buildCountMetrics(listLen, critLen, warnLen int) {
	if cd.addCountMetrics {
		cd.result.Metrics = append(cd.result.Metrics,
			&CheckMetric{
				Name:     "count",
				Value:    listLen,
				Warning:  cd.warnThreshold,
				Critical: cd.critThreshold,
				Min:      &Zero,
			},
		)
	}
	if cd.addProblemCountMetrics {
		cd.result.Metrics = append(cd.result.Metrics,
			&CheckMetric{
				Name:     "failed",
				Value:    critLen + warnLen,
				Warning:  cd.warnThreshold,
				Critical: cd.critThreshold,
				Min:      &Zero,
			},
		)
	}
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

	if state, ok := cd.details["_state"]; ok {
		cd.result.EscalateStatus(convert.Int64(state))
	}

	cd.details["_state"] = fmt.Sprintf("%d", cd.result.State)
}

// Check tries warn/crit/ok conditions against given data and sets result state.
func (cd *CheckData) Check(data map[string]string, warnCond, critCond, okCond ConditionList) {
	data["_state"] = fmt.Sprintf("%d", CheckExitOK)

	for i := range warnCond {
		if res, ok := warnCond[i].Match(data); res && ok {
			data["_state"] = fmt.Sprintf("%d", CheckExitWarning)
		}
	}

	for i := range critCond {
		if res, ok := critCond[i].Match(data); res && ok {
			data["_state"] = fmt.Sprintf("%d", CheckExitCritical)
		}
	}

	for i := range okCond {
		if res, ok := okCond[i].Match(data); res && ok {
			data["_state"] = fmt.Sprintf("%d", CheckExitOK)
		}
	}
}

// CheckMetrics tries warn/crit/ok conditions against given metrics and sets final state accordingly
func (cd *CheckData) CheckMetrics(warnCond, critCond, okCond ConditionList) {
	for _, metric := range cd.result.Metrics {
		state := CheckExitOK
		data := map[string]string{
			metric.Name: fmt.Sprintf("%v", metric.Value),
		}
		for i := range warnCond {
			if res, ok := warnCond[i].Match(data); res && ok {
				state = CheckExitWarning
			}
		}

		for i := range critCond {
			if res, ok := critCond[i].Match(data); res && ok {
				state = CheckExitCritical
			}
		}

		for i := range okCond {
			if res, ok := okCond[i].Match(data); res && ok {
				state = CheckExitOK
			}
		}
		if state > CheckExitOK {
			cd.result.EscalateStatus(state)
		}
	}
}

// MatchFilter returns true if {name: value} matches any filter
func (cd *CheckData) MatchFilter(name, value string) bool {
	data := map[string]string{name: value}
	res, _ := cd.MatchFilterMap(data)

	return res
}

// MatchFilterMap returns true if given map matches any filter
// returns either the result or not ok if the result cannt be determined
func (cd *CheckData) MatchFilterMap(data map[string]string) (res, ok bool) {
	finalOK := true
	for _, cond := range cd.filter {
		if cond.isNone {
			return true, true
		}
		if res, ok = cond.Match(data); res && ok {
			return true, true
		}
		if !ok {
			finalOK = false
		}
	}

	return false, finalOK
}

// MatchMapCondition returns true if listEntry matches filter
// preCheck defines behavior in case an attribute does not exist (set true for pre checks and false for final filter)
func (cd *CheckData) MatchMapCondition(conditions ConditionList, entry map[string]string, preCheck bool) bool {
	for _, cond := range conditions {
		if cond.isNone {
			continue
		}
		res, ok := cond.Match(entry)
		if !ok && !preCheck {
			res = cond.compareEmpty()
			ok = true
		}
		if !res && ok {
			return false
		}
	}

	return true
}

// Filter data map by conditions and return filtered list.
// ALl items not matching given filter will be removed.
func (cd *CheckData) Filter(conditions ConditionList, data []map[string]string) []map[string]string {
	if len(conditions) == 0 {
		return data
	}
	result := make([]map[string]string, 0)

	for num := range data {
		if cd.MatchMapCondition(conditions, data[num], false) {
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
func (cd *CheckData) ParseArgs(args []string) (argList []Argument, err error) {
	cd.rawArgs = args
	cd.hasArgsSupplied = map[string]bool{}
	argList = make([]Argument, 0, len(args))
	cd.expandArgDefinitions()
	topSupplied := false
	okSupplied := false
	sanitized, defaultWarning, defaultCritical, applyDefaultFilter, err := cd.preParseArgs(args)
	if err != nil {
		return nil, err
	}

	for _, arg := range sanitized {
		keyword := arg.key
		argValue := arg.value
		argExpr := arg.raw
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
			cond, err2 := NewCondition(argValue)
			if err2 != nil {
				return nil, err2
			}
			cd.okThreshold = append(cd.okThreshold, cond)
		case "warn+", "warning+":
			cd.warnThreshold = cd.fillDefaultThreshold(defaultWarning, cd.warnThreshold)

			fallthrough
		case "warn", "warning":
			cond, err2 := NewCondition(argValue)
			if err2 != nil {
				return nil, err2
			}
			cd.warnThreshold = append(cd.warnThreshold, cond)
		case "crit+", "critical+":
			cd.critThreshold = cd.fillDefaultThreshold(defaultCritical, cd.critThreshold)

			fallthrough
		case "crit", "critical":
			cond, err2 := NewCondition(argValue)
			if err2 != nil {
				return nil, err2
			}
			cd.critThreshold = append(cd.critThreshold, cond)
		case "filter+":
			cd.filter = cd.fillDefaultThreshold(cd.defaultFilter, cd.filter)

			fallthrough
		case "filter":
			applyDefaultFilter = false
			cond, err2 := NewCondition(argValue)
			if err2 != nil {
				return nil, err2
			}
			cd.filter = append(cd.filter, cond)
		case "debug":
			// not in use
		case "detail-syntax":
			cd.detailSyntax = argValue
		case "top-syntax":
			cd.topSyntax = argValue
			topSupplied = true
		case "ok-syntax":
			cd.okSyntax = argValue
			okSupplied = true
		case "empty-syntax":
			cd.emptySyntax = argValue
		case "empty-state":
			cd.emptyState = cd.parseStateString(argValue)
		case "show-all":
			if argValue == "" {
				cd.showAll = true
			} else {
				showAll, err2 := convert.BoolE(argValue)
				if err2 != nil {
					return nil, fmt.Errorf("parseBool %s: %s", argValue, err2.Error())
				}
				cd.showAll = showAll
			}
		case "timeout":
			timeout, err2 := convert.Float64E(argValue)
			if err2 != nil {
				return nil, fmt.Errorf("timeout parse error: %s", err2.Error())
			}
			cd.timeout = timeout
		case "perf-config":
			perf, err2 := NewPerfConfig(argValue)
			if err2 != nil {
				return nil, err2
			}
			cd.perfConfig = append(cd.perfConfig, perf...)
		case "perf-syntax":
			cd.perfSyntax = argValue
		case "output":
			cd.output = argValue
		default:
			parsed, err2 := cd.parseAnyArg(argExpr, keyword, argValue)
			switch {
			case err2 != nil:
				return nil, err2
			case parsed:
				// ok
			case cd.argsPassthrough:
				argList = append(argList, Argument{key: keyword, value: argValue})
			case keyword == "-h", keyword == "--help":
				cd.showHelp = PluginHelp
			default:
				return nil, fmt.Errorf("unknown argument: %s", keyword)
			}
		}
	}

	if topSupplied && !okSupplied {
		cd.okSyntax = cd.topSyntax
	}

	err = cd.setFallbacks(applyDefaultFilter, defaultWarning, defaultCritical)
	if err != nil {
		return nil, err
	}

	cd.applyConditionAlias()

	return argList, nil
}

func (cd *CheckData) preParseArgs(args []string) (sanitized []Argument, defaultWarning, defaultCritical string, hasArgsFilter bool, err error) {
	sanitized = make([]Argument, 0)
	numArgs := len(args)
	applyDefaultFilter := true
	for idx := 0; idx < numArgs; idx++ {
		argExpr := cd.removeQuotes(args[idx])
		split := strings.SplitN(argExpr, "=", 2)
		keyword := cd.removeQuotes(split[0])
		argValue, newIdx, err2 := cd.fetchNextArg(args, split, keyword, idx, numArgs)
		if err2 != nil {
			return nil, "", "", false, err2
		}
		idx = newIdx
		argValue = cd.removeQuotes(argValue)
		var chkArg *CheckArgument
		if a, ok := cd.args[keyword]; ok && a.isFilter {
			chkArg = &a
		}
		if a, ok := cd.extraArgs[keyword]; ok && a.isFilter {
			chkArg = &a
		}
		if chkArg != nil {
			applyDefaultFilter = false
			cd.hasArgsFilter = true
			if chkArg.defaultWarning != "" {
				defaultWarning = chkArg.defaultWarning
			}
			if chkArg.defaultCritical != "" {
				defaultCritical = chkArg.defaultCritical
			}
		}
		sanitized = append(sanitized, Argument{key: keyword, value: argValue, raw: argExpr})
	}

	return sanitized, defaultWarning, defaultCritical, applyDefaultFilter, nil
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
func (cd *CheckData) parseAnyArg(argExpr, keyword, argValue string) (bool, error) {
	arg, ok := cd.args[keyword]
	if !ok {
		arg, ok = cd.extraArgs[keyword]
		if !ok {
			return false, nil
		}
	}

	switch argRef := arg.value.(type) {
	case *[]string:
		if _, ok := cd.hasArgsSupplied[keyword]; !ok {
			// first time this arg occurs, empty default lists
			empty := make([]string, 0)
			*argRef = empty
		}
		*argRef = append(*argRef, argValue)
	case *CommaStringList:
		if _, ok := cd.hasArgsSupplied[keyword]; !ok {
			// first time this arg occurs, empty default lists
			empty := make([]string, 0)
			*argRef = empty
		}
		*argRef = append(*argRef, strings.Split(argValue, ",")...)
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
	case *int:
		i, err := strconv.ParseInt(argValue, 10, 32)
		if err != nil {
			return true, fmt.Errorf("parseInt %s: %s", argExpr, err.Error())
		}
		*argRef = int(i)
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

	cd.hasArgsSupplied[keyword] = true

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
func (cd *CheckData) setFallbacks(applyDefaultFilter bool, defaultWarning, defaultCritical string) error {
	if applyDefaultFilter && cd.defaultFilter != "" {
		cond, err := NewCondition(cd.defaultFilter)
		if err != nil {
			return err
		}
		cd.filter = append(cd.filter, cond)
	}

	// default warning/critical overridden from check arguments, ex. check_service
	if defaultWarning != "" {
		cd.defaultWarning = defaultWarning
	}
	if defaultCritical != "" {
		cd.defaultCritical = defaultCritical
	}

	cd.warnThreshold = cd.fillDefaultThreshold(cd.defaultWarning, cd.warnThreshold)
	cd.critThreshold = cd.fillDefaultThreshold(cd.defaultCritical, cd.critThreshold)

	if cd.timeout == 0 {
		cd.timeout = DefaultCheckTimeout
	}

	return nil
}

// HasMacro returns true is the syntax attributes contain a macro with the given name.
func (cd *CheckData) HasMacro(name string) bool {
	for _, syntax := range []string{cd.detailSyntax, cd.topSyntax, cd.okSyntax, cd.emptySyntax, cd.perfSyntax} {
		macros := MacroNames(syntax)
		if slices.Contains(macros, name) {
			return true
		}
	}

	return false
}

// apply condition aliases to all filter/warn/crit/ok conditions.
// this is useful for example in service checks, when people match for state running / started
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
func (cd *CheckData) applyConditionAliasList(condList ConditionList) {
	for _, cond := range condList {
		if len(cond.group) > 0 {
			cd.applyConditionAliasList(cond.group)

			continue
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
func (cd *CheckData) hasThresholdCond(condList ConditionList, name string) bool {
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
func (cd *CheckData) VisitAll(condList ConditionList, callback func(*Condition) bool) bool {
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

func (cd *CheckData) CloneThreshold(srcThreshold ConditionList) (cloned ConditionList) {
	cloned = make(ConditionList, 0)

	for i := range srcThreshold {
		cloned = append(cloned, srcThreshold[i].Clone())
	}

	return cloned
}

func (cd *CheckData) TransformThreshold(srcThreshold ConditionList, srcName, targetName, srcUnit, targetUnit string, total float64) (threshold ConditionList) {
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
func (cd *CheckData) TransformMultipleKeywords(srcKeywords []string, targetKeyword string, srcThreshold ConditionList) (threshold ConditionList) {
	transformed := cd.CloneThreshold(srcThreshold)
	applyChange := func(cond *Condition) bool {
		found := ""
		for _, keyword := range srcKeywords {
			if keyword == cond.keyword {
				found = keyword

				break
			}
		}
		if found == "" {
			return true
		}
		cond.keyword = targetKeyword
		switch {
		case strings.HasSuffix(found, "_pct"):
			cond.unit = "%"
		case strings.HasSuffix(found, "_bytes"):
			cond.unit = "B"
		}

		return true
	}
	cd.VisitAll(transformed, applyChange)

	return transformed
}

// replaces macros in threshold values, example as in: temperature > ${crit}
func (cd *CheckData) ExpandMetricMacros(srcThreshold ConditionList, data map[string]string) (threshold ConditionList) {
	replaced := cd.CloneThreshold(srcThreshold)
	applyChange := func(cond *Condition) bool {
		if cond.keyword == "" {
			return true
		}
		if cond.value == nil {
			return true
		}
		switch v := cond.value.(type) {
		case string:
			cond.value = ReplaceMacros(v, data)
		default:
		}

		return true
	}
	cd.VisitAll(replaced, applyChange)

	return replaced
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
	out := cd.helpHeader(format, false)
	out += cd.helpDefaultArguments(format)
	out += cd.helpSpecificArguments(format)
	out += cd.helpAttributes(format)

	out = strings.TrimSpace(out)

	return out
}

func (cd *CheckData) helpHeader(format ShowHelp, usageHeader bool) string {
	out := ""
	if cd.usage == "" {
		cd.usage = fmt.Sprintf("%s [<options>] [<filter>]", cd.name)
	}

	cd.exampleDefault = strings.TrimRight(cd.exampleDefault, " \t\n\r")
	cd.exampleDefault = strings.TrimLeft(cd.exampleDefault, "\n")

	if format == Markdown {
		out += cd.helpHeaderMarkdown(format, usageHeader)
	} else {
		out += fmt.Sprintf("Usage:\n\n    %s\n\n", cd.usage)
		out += fmt.Sprintf("    %s\n\n", cd.description)
		if cd.exampleDefault != "" {
			out += "Example:\n\n"
			out += fmt.Sprintf("%s\n\n", cd.exampleDefault)
		}
	}

	return out
}

func (cd *CheckData) helpHeaderMarkdown(format ShowHelp, usageHeader bool) string {
	out := ""
	title := strings.TrimPrefix(cd.name, "check_")
	if cd.docTitle != "" {
		title = cd.docTitle
	}
	out += fmt.Sprintf("---\ntitle: %s\n---\n\n", strings.TrimSpace(title))
	out += fmt.Sprintf("## %s\n\n", cd.name)
	out += fmt.Sprintf("%s\n\n", cd.description)
	out += "- [Examples](#examples)\n"
	if usageHeader {
		out += "- [Usage](#usage)\n"
	}
	if !cd.argsPassthrough {
		out += "- [Argument Defaults](#argument-defaults)\n"
	}
	if len(cd.attributes) > 0 {
		out += "- [Attributes](#attributes)\n"
	}
	out += "\n"

	if cd.implemented > 0 {
		out += cd.helpImplemented(format)
	}

	out += "## Examples\n\n"
	if cd.exampleDefault != "" {
		out += "### Default Check\n\n"
		out += fmt.Sprintf("%s\n\n", cd.exampleDefault)
	}
	out += "### Example using NRPE and Naemon\n\n"
	out += fmt.Sprintf(`Naemon Config

    define command{
        command_name         check_nrpe
        command_line         $USER1$/check_nrpe -H $HOSTADDRESS$ -n -c $ARG1$ -a $ARG2$
    }

    define service {
        host_name            testhost
        service_description  %s
        use                  generic-service
        check_command        check_nrpe!%s!%s
    }`, cd.name, cd.name, cd.exampleArgs)
	out += "\n\n"

	return out
}

func (cd *CheckData) isImplemented(platform string) bool {
	switch {
	case cd.implemented == ALL:
		return true
	case platform == "windows" && cd.implemented&Windows > 0:
		return true
	case platform == "linux" && cd.implemented&Linux > 0:
		return true
	case platform == "darwin" && cd.implemented&Darwin > 0:
		return true
	case platform == "freebsd" && cd.implemented&FreeBSD > 0:
		return true
	}

	return false
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

	if cd.argsPassthrough {
		return out
	}

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
		defaultArgs = append(defaultArgs, defaultArg{name: "critical", defaults: cd.defaultCritical})
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
	if len(cd.args) == 0 {
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
	if len(cd.attributes) == 0 {
		return out
	}

	if format == Markdown {
		out += "## Attributes\n\n"
		out += "### Filter Keywords\n\n"
	} else {
		out += "Filter Keywords:\n\n    "
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

// set default threshold unless already set
func (cd *CheckData) fillDefaultThreshold(defaultThreshold string, list ConditionList) ConditionList {
	if defaultThreshold == "" {
		return list
	}
	if len(list) > 0 {
		return list
	}
	condDef, err := NewCondition(defaultThreshold)
	if err != nil {
		log.Panicf("default threshold failed: %s", err.Error())
	}
	list = append(list, condDef)

	return list
}
