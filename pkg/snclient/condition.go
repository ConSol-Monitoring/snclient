package snclient

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/humanize"
	"github.com/consol-monitoring/snclient/pkg/utils"
)

var (
	reConditionValueUnit = regexp.MustCompile(`^(\-?\d+\.\d+|\-?\d+)\s*(\D+)$`)
	reCuddleKeyword      = regexp.MustCompile(`^([A-Za-z_]+)([!=><~]+)(.*)$`)
	reCuddleOperator     = regexp.MustCompile(`^([!=><~]+)(.*?)$`)
)

// Condition defines a condition as used in filters or thresholds
type Condition struct {
	noCopy noCopy

	keyword  string
	operator Operator
	value    interface{}
	unit     string

	// in case this is a group of conditions
	group         ConditionList
	groupOperator GroupOperator

	// if filter is a simple "none"
	isNone bool

	// store initial string
	original string

	// reference to check attributes (used to expand by unit)
	attr *[]CheckAttribute
}

// Operator defines a filter operator.
type Operator uint8

// Operator defines the operations available on filter
const (
	_ Operator = iota
	// Generic
	Equal   // =
	Unequal // !=

	// Text
	Contains            // like / ilike
	ContainsNot         // unlike / not ilike
	ContainsCase        // slike
	ContainsNotCase     // not slike
	RegexMatch          // ~
	RegexMatchNot       // !~
	RegexMatchNoCase    // ~~
	RegexMatchNotNoCase // !~~

	// Numeric
	Lower        // <
	LowerEqual   // <=
	Greater      // >
	GreaterEqual // >=

	// Lists
	InList    // in
	NotInList // not in
)

func OperatorParse(str string) (Operator, error) {
	switch strings.ToLower(str) {
	case "==", "=", "is", "eq":
		return Equal, nil
	case "!=", "is not", "ne":
		return Unequal, nil
	case "like", "ilike":
		return Contains, nil
	case "unlike", "not like", "not ilike":
		return ContainsNot, nil
	case "slike", "strictlike":
		return ContainsCase, nil
	case "not slike", "not strictlike":
		return ContainsNotCase, nil
	case "~", "regexp", "regex":
		return RegexMatch, nil
	case "!~", "not regex", "not regexp":
		return RegexMatchNot, nil
	case "~~", "regexpi", "regexi":
		return RegexMatchNoCase, nil
	case "!~~", "not regexi", "not regexpi":
		return RegexMatchNotNoCase, nil
	case "<", "lt":
		return Lower, nil
	case "<=", "le", "lte":
		return LowerEqual, nil
	case ">", "gt", "gte":
		return Greater, nil
	case ">=", "ge":
		return GreaterEqual, nil
	case "in":
		return InList, nil
	case "not in":
		return NotInList, nil
	}

	return 0, fmt.Errorf("unknown operator: %s", str)
}

func (o *Operator) String() string {
	switch *o {
	case Equal:
		return ("=")
	case Unequal:
		return ("!=")
	case Contains:
		return ("like")
	case ContainsNot:
		return ("unlike")
	case ContainsCase:
		return ("slike")
	case ContainsNotCase:
		return ("not slike")
	case RegexMatch:
		return ("~")
	case RegexMatchNot:
		return ("!~")
	case RegexMatchNoCase:
		return ("~~")
	case RegexMatchNotNoCase:
		return ("!~~")
	case Lower:
		return ("<")
	case LowerEqual:
		return ("<=")
	case Greater:
		return (">")
	case GreaterEqual:
		return (">=")
	case InList:
		return ("in")
	case NotInList:
		return ("not in")
	}

	return ("unknown")
}

// GroupOperator is the operator used to combine multiple filter conditions.
type GroupOperator uint8

// The only possible GroupOperator are "GroupAnd" and "GroupOr".
const (
	_ GroupOperator = iota
	GroupAnd
	GroupOr
)

func (g *GroupOperator) String() string {
	switch *g {
	case GroupAnd:
		return ("and")
	case GroupOr:
		return ("or")
	}

	return ("unknown")
}

// GroupOperatorParse parses group operator
func GroupOperatorParse(str string) (GroupOperator, error) {
	switch strings.ToLower(str) {
	case "and", "&&":
		return GroupAnd, nil
	case "or", "||":
		return GroupOr, nil
	}

	return 0, fmt.Errorf("unknown logical operator: %s", str)
}

// NewCondition parse filter= from check args
func NewCondition(input string, attr *[]CheckAttribute) (*Condition, error) {
	input = strings.TrimSpace(input)
	if input == "none" {
		return &Condition{isNone: true, original: input, attr: attr}, nil
	}

	token := utils.Tokenize(replaceStrOp(input))
	cond, remainingToken, err := conditionAdd(token, attr)
	if err != nil {
		return nil, err
	}
	if len(remainingToken) > 0 {
		return nil, fmt.Errorf("unexpected end of condition after '%s'", remainingToken[len(remainingToken)-1])
	}
	cond.original = input

	// convert /pattern/i regex into coresponding condition
	switch cond.operator { //nolint:exhaustive // only relevant for regex conditions
	case RegexMatch, RegexMatchNot, RegexMatchNoCase, RegexMatchNotNoCase:
		condStr := fmt.Sprintf("%v", cond.value)
		if strings.HasPrefix(condStr, "/") && strings.HasSuffix(condStr, "/i") {
			condStr = strings.TrimPrefix(condStr, "/")
			condStr = strings.TrimSuffix(condStr, "/i")
			cond.value = condStr
			switch cond.operator { //nolint:exhaustive // only relevant for regex conditions
			case RegexMatch:
				cond.operator = RegexMatchNoCase
			case RegexMatchNot:
				cond.operator = RegexMatchNotNoCase
			}
		} else if strings.HasPrefix(condStr, "/") && strings.HasSuffix(condStr, "/") {
			condStr = strings.TrimPrefix(condStr, "/")
			condStr = strings.TrimSuffix(condStr, "/")
			cond.value = condStr
		}
	}

	return cond, nil
}

func (c *Condition) String() string {
	if c.original != "" {
		return c.original
	}

	if len(c.group) > 0 {
		groups := []string{}
		for _, g := range c.group {
			groups = append(groups, g.String())
		}

		return "(" + strings.Join(groups, " "+c.groupOperator.String()+" ") + ")"
	}

	return fmt.Sprintf("%s %s %v%s", c.keyword, c.operator.String(), c.value, c.unit)
}

// Match checks if given map matches current condition
// returns either the result or not ok if the result cannot be determined because of none-existing values
func (c *Condition) Match(data map[string]string) (res, ok bool) {
	if c.isNone {
		return false, true
	}
	if len(c.group) > 0 {
		finalOK := true
		for i := range c.group {
			res, ok := c.group[i].Match(data)
			if !ok {
				finalOK = false

				continue
			}
			if !res && c.groupOperator == GroupAnd {
				return false, true
			}
			if res && c.groupOperator == GroupOr {
				return true, true
			}
		}

		// cannot make a deterministic decision
		if !finalOK {
			return false, false
		}

		// and: this means all conditions meet -> true.
		// or: it means no condition has met yet -> false
		return c.groupOperator == GroupAnd, true
	}

	return c.matchSingle(data)
}

// MatchAny checks if any given map matches current condition
func (c *Condition) MatchAny(data []map[string]string) (res, ok bool) {
	finalOK := false
	for i := range data {
		if res, ok = c.Match(data[i]); res && ok {
			return true, true
		}
		if ok {
			finalOK = true
		}
	}

	return false, finalOK
}

// MatchAnyOrEmpty checks if any given map matches current condition and falls back to empty condition
func (c *Condition) MatchAnyOrEmpty(data []map[string]string) (res bool) {
	res, ok := c.MatchAny(data)
	if !ok {
		res = c.compareEmpty()
	}

	return
}

// matchSingle checks a single condition and does not recurse into logical groups
// returns either the result or not ok if the value does not exist
func (c *Condition) matchSingle(data map[string]string) (res, ok bool) {
	if c.isNone {
		return true, true
	}
	varStr, ok := c.getVarValue(data)
	if !ok {
		return false, false
	}
	condStr := fmt.Sprintf("%v", c.value)
	varNum, err1 := strconv.ParseFloat(varStr, 64)
	condNum, err2 := strconv.ParseFloat(condStr, 64)
	if c.keyword == "version" {
		varNum, err1 = convert.VersionF64E(varStr)
		condNum, err2 = convert.VersionF64E(condStr)
	}
	switch c.operator {
	case Equal:
		if err1 == nil && err2 == nil {
			return varNum == condNum, true
		}
		// fallback to string compare
		return condStr == varStr, true
	case Unequal:
		if err1 == nil && err2 == nil {
			return varNum != condNum, true
		}
		// fallback to string compare
		return condStr != varStr, true
	case Contains:
		return strings.Contains(strings.ToLower(varStr), strings.ToLower(condStr)), true
	case ContainsNot:
		return !strings.Contains(strings.ToLower(varStr), strings.ToLower(condStr)), true
	case ContainsCase:
		return strings.Contains(varStr, condStr), true
	case ContainsNotCase:
		return !strings.Contains(varStr, condStr), true
	case GreaterEqual:
		if err1 == nil && err2 == nil {
			return varNum >= condNum, true
		}

		return false, true
	case Greater:
		if err1 == nil && err2 == nil {
			return varNum > condNum, true
		}

		return false, true
	case LowerEqual:
		if err1 == nil && err2 == nil {
			return varNum <= condNum, true
		}

		return false, true
	case Lower:
		if err1 == nil && err2 == nil {
			return varNum < condNum, true
		}

		return false, true
	case RegexMatch:
		regex, err := regexp.Compile(condStr)
		if err != nil {
			log.Warnf("invalid regex: %s: %s", condStr, err.Error())

			return false, true
		}

		return regex.MatchString(varStr), true
	case RegexMatchNot:
		regex, err := regexp.Compile(condStr)
		if err != nil {
			log.Warnf("invalid regex: %s: %s", condStr, err.Error())

			return false, true
		}

		return !regex.MatchString(varStr), true

	case RegexMatchNoCase:
		regex, err := regexp.Compile("(?i)" + condStr)
		if err != nil {
			log.Warnf("invalid regex: %s: %s", condStr, err.Error())

			return false, true
		}

		return regex.MatchString(varStr), true
	case RegexMatchNotNoCase:
		regex, err := regexp.Compile("(?i)" + condStr)
		if err != nil {
			log.Warnf("invalid regex: %s: %s", condStr, err.Error())

			return false, true
		}

		return !regex.MatchString(varStr), true

	case InList:
		if list, ok := c.value.([]string); ok {
			for _, el := range list {
				if el == varStr {
					return true, true
				}
			}
		}

		return false, true
	case NotInList:
		if list, ok := c.value.([]string); ok {
			for _, el := range list {
				if el == varStr {
					return false, true
				}
			}
		}

		return true, true
	}

	return false, true
}

// compareEmpty returns if the current condition operator is successful for an none-existing value.
// basically it returns false for positive comparisons and true for negative ones.
func (c *Condition) compareEmpty() bool {
	switch c.operator {
	case Equal,
		Contains,
		ContainsCase,
		GreaterEqual,
		Greater,
		RegexMatch,
		RegexMatchNoCase,
		InList:
		return false
	case Unequal,
		ContainsNot,
		ContainsNotCase,
		LowerEqual,
		Lower,
		RegexMatchNot,
		RegexMatchNotNoCase,
		NotInList:
		return true
	}

	return true
}

// getVarValue extracts value from dataset for conditions keyword
// tries keyword_pct for % unit and keyword_bytes for B unit
// returns value from keyword unless found already
func (c *Condition) getVarValue(data map[string]string) (varStr string, ok bool) {
	switch {
	case c.unit == "%":
		varStr, ok = data[c.keyword+"_pct"]
		if ok {
			return varStr, ok
		}
	case strings.EqualFold(c.unit, "B"):
		varStr, ok = data[c.keyword+"_bytes"]
		if ok {
			return varStr, ok
		}
	}

	varStr, ok = data[c.keyword+"_value"]
	if ok {
		return varStr, ok
	}

	varStr, ok = data[c.keyword]

	return varStr, ok
}

// Clone returns a new copy of this condition
func (c *Condition) Clone() *Condition {
	clone := &Condition{
		keyword:       c.keyword,
		operator:      c.operator,
		unit:          c.unit,
		value:         c.value,
		groupOperator: c.groupOperator,
		group:         make(ConditionList, 0),
		attr:          c.attr,
	}

	for i := range c.group {
		clone.group = append(clone.group, c.group[i].Clone())
	}

	return clone
}

// add parsed condition, returns remaining token
func conditionAdd(token []string, attr *[]CheckAttribute) (cond *Condition, remaining []string, err error) {
	if len(token) == 0 {
		return nil, nil, nil
	}

	conditions := make(ConditionList, 0)
	groupOp := GroupOperator(0)

	for len(token) > 0 {
		// closing bracket, return one level
		if token[0] == ")" {
			break
		}

		if len(conditions) > 0 {
			// we need an group operator first
			operator, err := GroupOperatorParse(token[0])
			if err != nil {
				return nil, nil, err
			}
			if len(token) == 1 {
				return nil, nil, fmt.Errorf("unexpected end of condition after '%s'", token[0])
			}
			token = token[1:]
			if groupOp != 0 && groupOp != operator {
				return nil, nil, fmt.Errorf("cannot mix logical operator in same block, use brackets")
			}
			groupOp = operator
		}

		// check if we start with a bracket
		if strings.HasPrefix(token[0], "(") {
			token[0] = strings.TrimPrefix(token[0], "(")
			// advance token if it was only the bracket itself
			if token[0] == "" {
				token = token[1:]
			}

			// parse sub group
			condsub, rem, err := conditionAdd(token, attr)
			if err != nil {
				return nil, nil, err
			}

			if len(rem) == 0 || rem[0] != ")" {
				return nil, nil, fmt.Errorf("expected closing bracket")
			}

			token = rem[1:] // excluding closing bracket
			conditions = append(conditions, condsub)

			continue
		}

		condsub, rem, err := conditionNext(token, attr)
		if err != nil {
			return nil, nil, err
		}
		token = rem
		conditions = append(conditions, condsub)
	}

	if len(conditions) == 1 {
		return conditions[0], token, nil
	}

	cond = &Condition{
		group:         conditions,
		groupOperator: groupOp,
		attr:          attr,
	}

	return cond, token, nil
}

// parse and remove next keyword/op/value combo from token list
func conditionNext(token []string, attr *[]CheckAttribute) (cond *Condition, remaining []string, err error) {
	keyword := token[0]
	token = token[1:]

	// keyword might cuddle with operator
	match := reCuddleKeyword.FindStringSubmatch(keyword)
	if len(match) > 0 {
		keyword = match[1]
		if match[3] == "" {
			token = append([]string{match[2]}, token...)
		} else {
			token = append([]string{match[2], match[3]}, token...)
		}
	}

	if len(token) == 0 {
		return nil, nil, fmt.Errorf("unexpected end of condition after '%s'", keyword)
	}
	query := keyword

	// operator might cuddle with value
	match = reCuddleOperator.FindStringSubmatch(token[0])
	if len(match) > 0 && match[2] != "" {
		token = append([]string{match[1], match[2]}, token[1:]...)
	}

	// trim quotes from keyword
	keyword, err = utils.TrimQuotes(keyword)
	if err != nil {
		return nil, nil, fmt.Errorf("%s", err.Error())
	}

	cond = &Condition{
		keyword: keyword,
		attr:    attr,
	}

	token = conditionFixTokenOperator(token)

	operator, err := OperatorParse(token[0])
	if err != nil {
		return nil, nil, err
	}
	query = query + " " + token[0]
	token = token[1:]
	cond.operator = operator

	if len(token) == 0 {
		return nil, nil, fmt.Errorf("expected value after '%s'", query)
	}

	rem, err := cond.conditionValue(token)
	if err != nil {
		return nil, nil, err
	}

	return cond, rem, nil
}

// parse and remove condition value
func (c *Condition) conditionValue(token []string) (remaining []string, err error) {
	// check for list values like ("a", "b",...)
	if strings.HasPrefix(token[0], "(") {
		rem, err2 := c.conditionListValue(token)
		if err2 != nil {
			return nil, err2
		}

		token = rem

		return token, nil
	}

	str := token[0]
	token = token[1:]

	// check for trailing closing brackets
	for strings.HasSuffix(str, ")") {
		str = strings.TrimSuffix(str, ")")
		token = append([]string{")"}, token...)
	}

	err = c.conditionSetValue(str, true)
	if err != nil {
		return nil, err
	}

	return token, nil
}

// parse and remove condition list value
func (c *Condition) conditionListValue(token []string) (remaining []string, err error) {
	token[0] = strings.TrimPrefix(token[0], "(")
	if token[0] == "" {
		token = token[1:]
	}

	// consume token until closing bracket
	list := []string{}
	for len(token) > 0 {
		str := token[0]
		token = token[1:]
		if strings.HasSuffix(str, ")") {
			str = strings.TrimSuffix(str, ")")
			if strings.HasSuffix(str, ",") {
				return nil, fmt.Errorf("trailing comma in value list after: %s", str)
			}
			if str != "" {
				list = append(list, str)
			}

			break
		}
		if !strings.HasSuffix(str, ",") && (len(token) == 0 || token[0] != ")") {
			return nil, fmt.Errorf("expected comma in value list after: %s", str)
		}
		if strings.HasSuffix(str, ",") && len(token) == 0 {
			return nil, fmt.Errorf("trailing comma in value list after: %s", str)
		}
		str = strings.TrimSuffix(str, ",")
		list = append(list, str)
	}

	// split by , and trim quotes
	res := []string{}
	for _, e := range list {
		subList := utils.TokenizeBy(e, ",", false, false)
		for _, elem := range subList {
			cond := &Condition{attr: c.attr}
			err = cond.conditionSetValue(elem, false)
			if err != nil {
				return nil, err
			}
			if v, ok := cond.value.(string); ok {
				res = append(res, v)
			}
		}
	}

	if len(res) == 0 {
		return nil, fmt.Errorf("empty value")
	}
	c.value = res

	return token, nil
}

// remove quotes and optionally expand known units
func (c *Condition) conditionSetValue(str string, expand bool) error {
	switch {
	case strings.HasPrefix(str, "'"):
		if !strings.HasSuffix(str, "'") || len(str) == 1 {
			return fmt.Errorf("unbalanced quotes in '%s'", str)
		}
		str = strings.TrimPrefix(str, "'")
		str = strings.TrimSuffix(str, "'")
		c.value = str

		return nil
	case strings.HasPrefix(str, `"`):
		if !strings.HasSuffix(str, `"`) || len(str) == 1 {
			return fmt.Errorf("unbalanced quotes in '%s'", str)
		}
		str = strings.TrimPrefix(str, `"`)
		str = strings.TrimSuffix(str, `"`)
		c.value = str

		return nil
	case !expand:
		c.value = str

		return nil
	default:
		return c.expandUnitByType(str)
	}
}

func (c *Condition) getUnit(keyword string) Unit {
	if c.attr == nil {
		return UNone
	}
	for _, k := range *c.attr {
		if strings.EqualFold(k.name, keyword) {
			return k.unit
		}
	}

	return UNone
}

func (c *Condition) expandUnitByType(str string) error {
	match := reConditionValueUnit.FindStringSubmatch(str)
	if len(match) < 3 {
		c.value = str

		return nil
	}
	c.value = match[1]
	c.unit = match[2]

	// bytes value support % thresholds as well but we cannot expand them yet
	if c.unit == "%" {
		return nil
	}

	// expand known units
	unit := c.getUnit(c.keyword)
	switch unit {
	case UByte:
		value, err := humanize.ParseBytes(str)
		if err != nil {
			return fmt.Errorf("invalid bytes value: %s", err.Error())
		}
		c.value = strconv.FormatUint(value, 10)
		c.unit = "B"

		return nil
	case UDate, UTimestamp:
		value, err := utils.ExpandDuration(str)
		if err != nil {
			return fmt.Errorf("invalid duration value: %s", err.Error())
		}
		c.value = strconv.FormatFloat(float64(time.Now().Unix())+value, 'f', 0, 64)
		c.unit = ""

		return nil
	case UDuration:
		value, err := utils.ExpandDuration(str)
		if err != nil {
			return fmt.Errorf("invalid duration value: %s", err.Error())
		}
		c.value = strconv.FormatFloat(value, 'f', 0, 64)
		c.unit = "s"

		return nil
	case UPercent:
		return nil
	case UNone:
		// best effort unit expansion
		return c.expandUnitByName(str)
	}

	return nil
}

func (c *Condition) expandUnitByName(str string) error {
	// best effort unit expansion
	switch strings.ToLower(c.unit) {
	case "kb", "mb", "gb", "tb", "pb", "kib", "mib", "gib", "tib", "pib":
		value, err := humanize.ParseBytes(str)
		if err != nil {
			return fmt.Errorf("invalid bytes value: %s", err.Error())
		}
		c.value = strconv.FormatUint(value, 10)
		c.unit = "B"
	case "ms", "h", "d", "w", "y": // do not expand "m" here, as it is ambiguous
		value, err := utils.ExpandDuration(str)
		if err != nil {
			return fmt.Errorf("invalid duration value: %s", err.Error())
		}
		c.value = strconv.FormatFloat(value, 'f', 0, 64)
		c.unit = "s"
	}

	return nil
}

// fix some corner cases in token lists, ex.:
func conditionFixTokenOperator(token []string) []string {
	if len(token) >= 2 {
		switch {
		// append "not" to next token
		case strings.EqualFold(token[0], "not"):
			token[1] = "not " + token[1]
			token = token[1:]
		// append "not" to previous  token
		case strings.EqualFold(token[1], "not"):
			token[1] = token[0] + " not"
			token = token[1:]
		}

		// support regex matches of form: attr ~~ /value/modifier
		switch token[0] {
		case "~", "~~", "!~", "!~~":
			if strings.HasPrefix(token[1], "/") {
				// consume all remaining token till an ending / is found
				for len(token) > 2 {
					token[1] = token[1] + " " + token[2]
					token = append(token[:2], token[3:]...)
					// check if token now ends with / or /i - we only support i option so far
					if strings.HasSuffix(token[1], "/") || strings.HasSuffix(token[1], "/i") {
						break
					}
				}
			}
		default:
			// keep like it is
		}
	}

	// separate function call from options
	if len(token) >= 1 {
		switch {
		case strings.HasPrefix(strings.ToLower(token[0]), "in("):
			token[0] = token[0][2:]
			token = append([]string{"in"}, token...)
		case strings.HasPrefix(strings.ToLower(token[0]), "not in("):
			token[0] = token[0][6:]
			token = append([]string{"not in"}, token...)
		}
	}

	return token
}

// ThresholdString returns string used in warn/crit threshold performance data.
func ThresholdString(name []string, conditions ConditionList, numberFormat func(interface{}) string) string {
	// fetch warning conditions for name of metric
	filtered := make(ConditionList, 0)
	var group GroupOperator
	for num := range conditions {
		if slices.Contains(name, conditions[num].keyword) {
			filtered = append(filtered, conditions[num])
		}
		if conditions[num].groupOperator == GroupOr {
			group = conditions[num].groupOperator
			for i := range conditions[num].group {
				if slices.Contains(name, conditions[num].group[i].keyword) {
					filtered = append(filtered, conditions[num].group[i])
				}
			}
		}
		if conditions[num].groupOperator == GroupAnd {
			group = conditions[num].groupOperator
			for i := range conditions[num].group {
				if slices.Contains(name, conditions[num].group[i].keyword) {
					filtered = append(filtered, conditions[num].group[i])
				}
			}
		}
	}

	if len(filtered) == 0 {
		return ""
	}

	if len(filtered) == 1 {
		//exhaustive:ignore // only the lower conditions get a trailing ":"
		switch filtered[0].operator {
		case Lower:
			return numberFormat(filtered[0].value) + ":"
		case LowerEqual:
			thisNumber, _ := convert.Float64E(filtered[0].value)
			nextNumber := math.Ceil(thisNumber)
			if thisNumber == nextNumber {
				nextNumber++
			}

			return numberFormat(nextNumber) + ":"
		default:
			return numberFormat(filtered[0].value)
		}
	}

	if len(filtered) == 2 {
		low := filtered[0].value
		high := filtered[1].value
		num1, err1 := convert.Float64E(low)
		num2, err2 := convert.Float64E(high)
		if err1 != nil || err2 != nil {
			return ""
		}
		// switch numbers
		if num1 > num2 {
			low = filtered[1].value
			high = filtered[0].value
		}
		if group == GroupOr {
			return fmt.Sprintf("%s:%s", numberFormat(low), numberFormat(high))
		}

		// implicite And
		return fmt.Sprintf("@%s:%s", numberFormat(low), numberFormat(high))
	}

	return ""
}

// list of conditions
type ConditionList []*Condition

func (cl *ConditionList) String() string {
	if len(*cl) == 0 {
		return ("none")
	}

	if len(*cl) == 1 {
		return (*cl)[0].String()
	}

	res := []string{}
	for _, c := range *cl {
		res = append(res, c.String())
	}

	// top level conditions are joined as OR
	return strings.Join(res, " or ")
}

func replaceStrOp(input string) string {
	token := utils.TokenizeBy(input, "()", true, true)

	output := make([]string, 0, len(token))
	for idx := 0; idx < len(token); idx++ {
		str := token[idx]
		if strings.HasSuffix(str, "str") && len(token) >= idx+3 && token[idx+1] == "(" {
			// str()
			if token[idx+2] == ")" {
				output = append(output, strings.TrimSuffix(str, "str"), "''")
				idx += 2

				continue
			}

			// str(')
			if strings.HasSuffix(token[idx+2], ")") {
				output = append(output, strings.TrimSuffix(str, "str"), "'"+strings.ReplaceAll(strings.TrimSuffix(token[idx+2], ")"), "'", "\\'")+"'")
				idx += 2

				continue
			}

			// str(txt)
			if len(token) >= idx+4 && token[idx+3] == ")" {
				output = append(output, strings.TrimSuffix(str, "str"), "'"+strings.ReplaceAll(token[idx+2], "'", "\\'")+"'")
				idx += 3

				continue
			}
		}

		output = append(output, str)
	}

	return strings.Join(output, "")
}
