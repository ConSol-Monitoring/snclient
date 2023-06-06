package snclient

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"pkg/convert"
	"pkg/humanize"
	"pkg/utils"
)

var (
	reConditionValueUnit = regexp.MustCompile(`^(\d+\.\d+|\d+)\s*(\D+)$`)
	reConditionKeyword   = regexp.MustCompile(`^([A-Za-z_]+)([!=><]+)(.+)$`)
)

// Condition defines a condition as used in filters or thresholds
type Condition struct {
	noCopy noCopy

	keyword  string
	operator Operator
	value    interface{}
	unit     string

	// in case this is a group of conditions
	group         []*Condition
	groupOperator GroupOperator

	// if filter is a simple "none"
	isNone bool
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
	Contains      // like
	ContainsNot   // unlike
	RegexMatch    // ~
	RegexMatchNot // !~

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
	case "like":
		return Contains, nil
	case "unlike", "not like":
		return ContainsNot, nil
	case "~", "regexp", "regex":
		return RegexMatch, nil
	case "!~", "not regex", "not regexp":
		return RegexMatchNot, nil
	case "<", "lt":
		return Lower, nil
	case "<=", "le":
		return LowerEqual, nil
	case ">", "gt":
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

// GroupOperator is the operator used to combine multiple filter conditions.
type GroupOperator uint8

// The only possible GroupOperator are "GroupAnd" and "GroupOr".
const (
	_ GroupOperator = iota
	GroupAnd
	GroupOr
)

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
func NewCondition(input string) (*Condition, error) {
	input = strings.TrimSpace(input)
	if input == "none" {
		return &Condition{isNone: true}, nil
	}

	token := utils.Tokenize(input)
	cond, remainingToken, err := conditionAdd(token)
	if err != nil {
		return nil, err
	}
	if len(remainingToken) > 0 {
		return nil, fmt.Errorf("unexpected end of condition after '%s'", remainingToken[len(remainingToken)-1])
	}

	return cond, nil
}

// Match checks if given map matches current condition
func (c *Condition) Match(data map[string]string) bool {
	if c.isNone {
		return false
	}
	if len(c.group) > 0 {
		for i := range c.group {
			res := c.group[i].matchSingle(data)
			if !res && c.groupOperator == GroupAnd {
				return false
			}
			if res && c.groupOperator == GroupOr {
				return true
			}
		}

		// and: this means all conditions meet -> true.
		// or: it means no condition has met yet -> false
		return c.groupOperator == GroupAnd
	}

	return c.matchSingle(data)
}

// matchSingle checks a single condition and does not recurse into logical groups
func (c *Condition) matchSingle(data map[string]string) bool {
	if c.isNone {
		return true
	}
	varStr, ok := c.getVarValue(data)
	if !ok {
		return false
	}
	condStr := fmt.Sprintf("%v", c.value)
	varNum, err1 := strconv.ParseFloat(varStr, 64)
	condNum, err2 := strconv.ParseFloat(condStr, 64)
	switch c.operator {
	case Equal:
		if err1 == nil && err2 == nil {
			return varNum == condNum
		}
		// fallback to string compare
		return condStr == varStr
	case Unequal:
		if err1 == nil && err2 == nil {
			return varNum != condNum
		}
		// fallback to string compare
		return condStr != varStr
	case Contains:
		return strings.Contains(varStr, condStr)
	case ContainsNot:
		return !strings.Contains(varStr, condStr)
	case GreaterEqual:
		if err1 == nil && err2 == nil {
			return varNum >= condNum
		}

		return false
	case Greater:
		if err1 == nil && err2 == nil {
			return varNum > condNum
		}

		return false
	case LowerEqual:
		if err1 == nil && err2 == nil {
			return varNum <= condNum
		}

		return false
	case Lower:
		if err1 == nil && err2 == nil {
			return varNum < condNum
		}

		return false
	case RegexMatch:
		regex, err := regexp.Compile(condStr)
		if err != nil {
			log.Warnf("invalid regex: %s: %s", condStr, err.Error())

			return false
		}

		return regex.MatchString(varStr)
	case RegexMatchNot:
		regex, err := regexp.Compile(condStr)
		if err != nil {
			log.Warnf("invalid regex: %s: %s", condStr, err.Error())

			return false
		}

		return !regex.MatchString(varStr)
	case InList:
		if list, ok := c.value.([]string); ok {
			for _, el := range list {
				if el == varStr {
					return true
				}
			}
		}

		return false
	case NotInList:
		if list, ok := c.value.([]string); ok {
			for _, el := range list {
				if el == varStr {
					return false
				}
			}
		}

		return true
	}

	return false
}

// getVarValue extracts value from dataset for conditions keyword
// tries keyword_pct for % unit and keyword_bytes for B unit
// returns value from keyword unless found already
func (c *Condition) getVarValue(data map[string]string) (val string, ok bool) {
	switch {
	case c.unit == "%":
		varStr, ok := data[c.keyword+"_pct"]
		if ok {
			return varStr, ok
		}
	case strings.EqualFold(c.unit, "B"):
		varStr, ok := data[c.keyword+"_bytes"]
		if ok {
			return varStr, ok
		}
	}

	varStr, ok := data[c.keyword]

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
		group:         make([]*Condition, 0),
	}

	for i := range c.group {
		clone.group = append(clone.group, c.group[i].Clone())
	}

	return clone
}

// add parsed condition, returns remaining token
func conditionAdd(token []string) (cond *Condition, remaining []string, err error) {
	if len(token) == 0 {
		return nil, nil, nil
	}

	conditions := make([]*Condition, 0)
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
			cond, rem, err := conditionAdd(token)
			if err != nil {
				return nil, nil, err
			}

			if len(rem) == 0 || rem[0] != ")" {
				return nil, nil, fmt.Errorf("expected closing bracket")
			}

			token = rem[1:] // excluding closing bracket
			conditions = append(conditions, cond)

			continue
		}

		cond, rem, err := conditionNext(token)
		if err != nil {
			return nil, nil, err
		}
		token = rem
		conditions = append(conditions, cond)
	}

	if len(conditions) == 1 {
		return conditions[0], token, nil
	}

	cond = &Condition{
		group:         conditions,
		groupOperator: groupOp,
	}

	return cond, token, nil
}

// parse and remove next keyword/op/value combo from token list
func conditionNext(token []string) (cond *Condition, remaining []string, err error) {
	keyword := token[0]
	token = token[1:]

	// keyword might cuddle with operator
	match := reConditionKeyword.FindStringSubmatch(keyword)
	if len(match) > 0 {
		keyword = match[1]
		token = append([]string{match[2], match[3]}, token...)
	}

	if len(token) == 0 {
		return nil, nil, fmt.Errorf("unexpected end of condition after '%s'", keyword)
	}
	query := keyword

	cond = &Condition{
		keyword: keyword,
	}

	// token might consist of 2 token, try that first
	if len(token) > 1 {
		operator2, err2 := OperatorParse(token[0] + " " + token[1])
		if err2 == nil {
			query = query + " " + token[0] + " " + token[1]
			token = token[2:]
			cond.operator = operator2
		}
	}

	// no operator yet?
	if cond.operator == 0 {
		operator, err := OperatorParse(token[0])
		if err != nil {
			return nil, nil, err
		}
		query = query + " " + token[0]
		token = token[1:]
		cond.operator = operator
	}

	if len(token) == 0 {
		return nil, nil, fmt.Errorf("expected value after '%s'", query)
	}

	rem, err := conditionValue(cond, token)
	if err != nil {
		return nil, nil, err
	}

	return cond, rem, nil
}

// parse and remove condition value
func conditionValue(cond *Condition, token []string) (remaining []string, err error) {
	// check for list values like ("a", "b",...)
	if strings.HasPrefix(token[0], "(") {
		rem, err := conditionListValue(cond, token)
		if err != nil {
			return nil, err
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

	err = conditionSetValue(cond, str, true)
	if err != nil {
		return nil, err
	}

	return token, nil
}

// parse and remove condition list value
func conditionListValue(cond *Condition, token []string) (remaining []string, err error) {
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
		subList := utils.TokenizeBy(e, ",")
		for _, elem := range subList {
			c := &Condition{}
			err = conditionSetValue(c, elem, false)
			if err != nil {
				return nil, err
			}
			if v, ok := c.value.(string); ok {
				res = append(res, v)
			}
		}
	}

	if len(res) == 0 {
		return nil, fmt.Errorf("empty value")
	}
	cond.value = res

	return token, nil
}

// remove quotes and optionally expand known units
func conditionSetValue(cond *Condition, str string, expand bool) error {
	switch {
	case strings.HasPrefix(str, "'"):
		if !strings.HasSuffix(str, "'") || len(str) == 1 {
			return fmt.Errorf("unbalanced quotes in '%s'", str)
		}
		str = strings.TrimPrefix(str, "'")
		str = strings.TrimSuffix(str, "'")
		cond.value = str
	case strings.HasPrefix(str, `"`):
		if !strings.HasSuffix(str, `"`) || len(str) == 1 {
			return fmt.Errorf("unbalanced quotes in '%s'", str)
		}
		str = strings.TrimPrefix(str, `"`)
		str = strings.TrimSuffix(str, `"`)
		cond.value = str
	case !expand:
		cond.value = str
	default:
		match := reConditionValueUnit.FindStringSubmatch(str)
		if len(match) == 3 {
			cond.value = match[1]
			cond.unit = match[2]

			// expand known units
			switch strings.ToLower(cond.unit) {
			case "kb", "mb", "gb", "tb", "pb",
				"kib", "mib", "gib", "tib", "pib":
				value, _ := humanize.ParseBytes(str)
				cond.value = strconv.FormatUint(value, 10)
				cond.unit = "B"
			case "m", "h", "d":
				value, _ := utils.ExpandDuration(str)
				cond.value = strconv.FormatFloat(value, 'f', 0, 64)
				cond.unit = "s"
			}
		} else {
			cond.value = str
		}
	}

	return nil
}

// ThresholdString returns string used in warn/crit threshold performance data.
//
//nolint:funlen // allow more than 40 statements
func ThresholdString(name string, conditions []*Condition) string {
	// fetch warning conditions for name of metric
	filtered := make([]*Condition, 0)
	var group GroupOperator
	for num := range conditions {
		if conditions[num].keyword == name {
			filtered = append(filtered, conditions[num])
		}
		if conditions[num].groupOperator == GroupOr {
			group = conditions[num].groupOperator
			for i := range conditions[num].group {
				if conditions[num].group[i].keyword == name {
					filtered = append(filtered, conditions[num].group[i])
				}
			}
		}
		if conditions[num].groupOperator == GroupAnd {
			group = conditions[num].groupOperator
			for i := range conditions[num].group {
				if conditions[num].group[i].keyword == name {
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
			return convert.Num2String(filtered[0].value) + ":"
		case LowerEqual:
			thisNumber, _ := convert.Float64E(filtered[0].value)
			nextNumber := math.Ceil(thisNumber)
			if thisNumber == nextNumber {
				nextNumber++
			}

			return convert.Num2String(nextNumber) + ":"
		default:
			return convert.Num2String(filtered[0].value)
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
		if num1 > num2 {
			low = filtered[1].value
			high = filtered[0].value
		}
		if group == GroupOr {
			return fmt.Sprintf("%s:%s", convert.Num2String(low), convert.Num2String(high))
		}
		if group == GroupAnd {
			return fmt.Sprintf("@%s:%s", convert.Num2String(low), convert.Num2String(high))
		}
	}

	return ""
}
