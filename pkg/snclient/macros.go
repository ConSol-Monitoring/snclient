package snclient

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/humanize"
	"github.com/consol-monitoring/snclient/pkg/utils"
	"github.com/sni/shelltoken"
)

const (
	macroChars        = `a-zA-Z0-9\-_ /.`
	runtimeMacroChars = `a-zA-Z0-9"`
)

var (
	// macros can be either ${...} or %(...) or all variants of $ / % with () or {}
	reOnDemandMacro = regexp.MustCompile(`(\$|%)(\{\s*[` + macroChars + `]+\s*\}|\(\s*[` + macroChars + `]+\s*\))`)

	// runtime macros can be %...% or $...$ or $ARGS"$
	reRuntimeMacro = regexp.MustCompile(`%[` + runtimeMacroChars + `]+%|\$[` + runtimeMacroChars + `]+\$`)

	reFloatFormat = regexp.MustCompile(`^%[.\d]*f$`)

	reASCIIonly = regexp.MustCompile(`\W`)

	macroSplitBy = map[string]string{
		"$(": ")",
		"${": "}",
		"%(": ")",
		"%{": "}",
	}
)

// ReplaceTemplate combines ReplaceConditionals and ReplaceMacros
func ReplaceTemplate(value string, timezone *time.Location, macroSets ...map[string]string) (string, error) {
	expanded, err := ReplaceConditionals(value, macroSets...)
	if err != nil {
		return expanded, err
	}
	expanded = ReplaceMacros(expanded, timezone, macroSets...)

	return expanded, nil
}

/* ReplaceConditionals replaces conditionals of the form
 * {{ IF condition }}...{{ ELSIF condition }}...{{ ELSE }}...{{ END }}"
 */
func ReplaceConditionals(value string, macroSets ...map[string]string) (string, error) {
	splitBy := map[string]string{
		"{{": "}}",
	}
	token, err := splitToken(value, splitBy)
	if err != nil {
		return value, fmt.Errorf("replacing conditionals in %s failed: %s", value, err.Error())
	}

	type state struct {
		curOK     bool
		completed bool
	}
	condState := []state{}
	var curState *state

	var result strings.Builder
	for _, piece := range token {
		condFound := false
		for startPattern, endPattern := range splitBy {
			if !strings.HasPrefix(piece, startPattern) || !strings.HasSuffix(piece, endPattern) {
				continue
			}
			// orig := piece
			piece = strings.TrimPrefix(piece, startPattern)
			piece = strings.TrimSuffix(piece, endPattern)
			piece = strings.TrimSpace(piece)
			fields := utils.FieldsN(piece, 2)

			switch strings.ToLower(fields[0]) {
			case "if":
				if len(fields) < 2 {
					return value, fmt.Errorf("missing condition in %s clause :%s", strings.ToUpper(fields[0]), piece)
				}
				condition, err := NewCondition(fields[1], nil)
				if err != nil {
					return value, fmt.Errorf("parsing condition in %s failed: %s", fields[1], err.Error())
				}

				curState = &state{curOK: condition.MatchAnyOrEmpty(macroSets)}
				condState = append(condState, *curState)
			case "elsif":
				if curState == nil {
					return value, fmt.Errorf("unexpected ELSIF in: %s", value)
				}
				if len(fields) < 2 {
					return value, fmt.Errorf("missing condition in %s clause :%s", strings.ToUpper(fields[0]), piece)
				}
				if curState.curOK || curState.completed {
					curState.completed = true
					curState.curOK = false

					break
				}
				condition, err := NewCondition(fields[1], nil)
				if err != nil {
					return value, fmt.Errorf("parsing condition in %s failed: %s", fields[1], err.Error())
				}

				curState.curOK = condition.MatchAnyOrEmpty(macroSets)
			case "else":
				if curState.curOK || curState.completed {
					curState.completed = true
					curState.curOK = false

					break
				}
				curState.curOK = true
			case "end":
				if curState == nil {
					return value, fmt.Errorf("unexpected END in: %s", value)
				}
				condState = condState[0 : len(condState)-1]
				curState = nil
				if len(condState) > 0 {
					curState = &condState[len(condState)-1]
				}
			}

			condFound = true

			break
		}
		if condFound {
			continue
		}

		if curState == nil || curState.curOK {
			result.WriteString(piece)
		}
	}

	return result.String(), nil
}

// MacroNames returns list of used macros.
func MacroNames(text string) []string {
	list := []string{}
	uniq := map[string]bool{}

	token, err := splitToken(text, macroSplitBy)
	if err != nil {
		return list
	}

	for _, piece := range token {
		for startPattern, endPattern := range macroSplitBy {
			if !strings.HasPrefix(piece, startPattern) || !strings.HasSuffix(piece, endPattern) {
				continue
			}
			piece = strings.TrimPrefix(piece, startPattern)
			piece = strings.TrimSuffix(piece, endPattern)
			piece = strings.TrimSpace(piece)

			if _, ok := uniq[piece]; !ok {
				uniq[piece] = true
				list = append(list, piece)
			}

			break
		}
	}

	return list
}

/* replaceMacros replaces variables in given string (config ini file style macros).
 * possible macros are:
 *   ${macro} / $(macro)
 *   %(macro) / %{macro}
 */
func ReplaceMacros(value string, timezone *time.Location, macroSets ...map[string]string) string {
	token, err := splitToken(value, macroSplitBy)
	if err != nil {
		log.Errorf("replacing macros in %s failed: %s", value, err.Error())

		return value
	}

	var result strings.Builder
	for _, piece := range token {
		inMacro := false
		for startPattern, endPattern := range macroSplitBy {
			if !strings.HasPrefix(piece, startPattern) || !strings.HasSuffix(piece, endPattern) {
				continue
			}
			orig := piece
			piece = strings.TrimPrefix(piece, startPattern)
			piece = strings.TrimSuffix(piece, endPattern)
			piece = strings.TrimSpace(piece)

			result.WriteString(getMacrosetsValue(piece, orig, timezone, macroSets...))
			inMacro = true

			break
		}

		if !inMacro {
			result.WriteString(piece)
		}
	}

	return result.String()
}

/* ReplaceRuntimeMacros replaces runtime variables in given string (check output template style macros).
 * possible macros are:
 *   %macro%
 *   $macro$
 */
func ReplaceRuntimeMacros(value string, timezone *time.Location, macroSets ...map[string]string) string {
	value = reRuntimeMacro.ReplaceAllStringFunc(value, func(str string) string {
		orig := str
		str = strings.TrimSpace(str)

		switch {
		// %...% macros
		case strings.HasPrefix(str, "%"):
			str = strings.TrimPrefix(str, "%")
			str = strings.TrimSuffix(str, "%")
		// $...$ macros
		case strings.HasPrefix(str, "$"):
			str = strings.TrimPrefix(str, "$")
			str = strings.TrimSuffix(str, "$")
		}

		return getMacrosetsValue(str, orig, timezone, macroSets...)
	})

	return value
}

func extractMacroString(str string) string {
	str = strings.TrimSpace(str)

	switch {
	// $... macros
	case strings.HasPrefix(str, "$"):
		str = strings.TrimPrefix(str, "$")
	// %... macros
	case strings.HasPrefix(str, "%"):
		str = strings.TrimPrefix(str, "%")
	}

	switch {
	// {...} macros
	case strings.HasPrefix(str, "{"):
		str = strings.TrimPrefix(str, "{")
		str = strings.TrimSuffix(str, "}")
	// (...) macros
	case strings.HasPrefix(str, "("):
		str = strings.TrimPrefix(str, "(")
		str = strings.TrimSuffix(str, ")")
		str = strings.TrimSpace(str)
	}

	return (str)
}

func getMacrosetsValue(macro, orig string, timezone *time.Location, macroSets ...map[string]string) string {
	// split by : and |
	flags := strings.FieldsFunc(macro, func(r rune) bool { return r == '|' || r == ':' })

	macro = strings.TrimSpace(flags[0])
	value := orig

	found := false
	// strip off common suffixes from marco name
	for _, suffix := range []string{"", "_unix"} {
		macroName := strings.TrimSuffix(macro, suffix)
		for _, ms := range macroSets {
			if repl, ok := ms[macroName]; ok {
				value = repl
				found = true

				break
			}
		}
	}

	// if no macro replacement was found, do not convert flags
	if !found {
		return value
	}

	// no macro operator present
	if len(flags) == 1 {
		return value
	}

	return (replaceMacroOperators(value, flags[1:], timezone))
}

func replaceMacroOperators(value string, flags []string, timezone *time.Location) string {
	for _, flag := range flags {
		flag = strings.TrimSpace(flag)

		switch flag {
		// lc -> lowercase
		case "lc":
			value = strings.ToLower(value)
		// uc -> lowercase
		case "uc":
			value = strings.ToUpper(value)
		// h -> human readable number
		case "h":
			value = humanize.NumF(convert.Int64(value), 2)
		// date -> unix timestamp to date with local timezone
		case "date":
			value = time.Unix(convert.Int64(value), 0).In(timezone).Format("2006-01-02 15:04:05 MST")
		// date -> unix timestamp to utc date
		case "utc":
			value = time.Unix(convert.Int64(value), 0).UTC().Format("2006-01-02 15:04:05 MST")
		// duration -> seconds into duration string
		case "duration":
			value = utils.DurationString(time.Duration(convert.Float64(value) * float64(time.Second)))
		case "age":
			value = fmt.Sprintf("%d", time.Now().Unix()-convert.Int64(value))
		case "ascii":
			value = reASCIIonly.ReplaceAllString(value, "")
		case "trim":
			value = strings.TrimSpace(value)
		case "chomp":
			value = strings.TrimRight(value, " \t\n\r")
		default:
			value = replaceMacroOpString(value, flag)
		}
	}

	return value
}

func replaceMacroOpString(value, flag string) string {
	flag, _ = utils.TrimQuotes(flag)

	switch {
	// number format fmt=...
	case strings.HasPrefix(flag, "fmt="):
		format := strings.TrimPrefix(flag, "fmt=")
		switch {
		case format == "%d":
			value = fmt.Sprintf(format, int64(convert.Float64(value)))
		case reFloatFormat.MatchString(format):
			value = fmt.Sprintf(format, convert.Float64(value))
		default:
			log.Warnf("unsupported format string used: %s", format)
		}
	case strings.HasPrefix(flag, "cut="):
		// Expect cut=%d+ (number of chars)
		cut, err := strconv.Atoi(strings.TrimPrefix(flag, "cut="))
		runes := []rune(value)
		if err != nil {
			log.Warn("could not extract cut macro expected format cut=%d+")

			return ""
		}
		if cut > len(runes) {
			return value
		}
		value = string(runes[0:cut])
	case strings.HasPrefix(flag, "s/"):
		token, err := shelltoken.SplitQuotes(flag[1:], "/", shelltoken.SplitKeepBackslashes|shelltoken.SplitKeepQuotes|shelltoken.SplitKeepSeparator)
		if err != nil {
			log.Warnf("regexp syntax error, format s/pattern/replacement/ -> %s: %s", flag, err.Error())

			return value
		}
		if len(token) < 4 {
			log.Warnf("regexp syntax error, format s/pattern/replacement/ -> %s", flag)

			return value
		}
		pattern := token[1]
		replace := token[3]
		if replace == "/" {
			replace = ""
		}
		if pattern == "/" {
			log.Warnf("regexp syntax error, format s/pattern/replacement/ -> %s", flag)

			return value
		}
		regex, err := regexp.Compile(pattern)
		if err != nil {
			log.Warnf("regexp compile error %s: %s", pattern, err.Error())

			return value
		}

		value = regex.ReplaceAllString(value, replace)
	default:
		log.Warnf("unknown macro processor: %s", flag)

		return value
	}

	return value
}

// splitToken splits text into pieces that start with the key and end with the value of the token map
//
// ex.: splitToken("...'$(macro)'...", map[string]string{"$(": ")"})
// ->[]string{"...'", "$(macro)", "'..."}
func splitToken(input string, token map[string]string) (splitted []string, err error) {
	inToken := ""
	tokenStart := 0
	inDoubleQuotes := false
	inSingleQuotes := false
	escaped := false
	for pos := range input {
		if pos < tokenStart {
			// catch up with already added parts
			continue
		}
		if inToken == "" {
			// does a token start at this position
			for tStart, tEnd := range token {
				if !strings.HasPrefix(input[pos:], tStart) {
					continue
				}
				prefix := input[tokenStart:pos]
				if prefix != "" {
					splitted = append(splitted, input[tokenStart:pos])
				}

				inToken = tEnd
				tokenStart = pos

				break
			}
		} else {
			// inside the token we honor quote and escape characters
			char := input[pos]
			switch {
			case escaped:
				escaped = false
			case char == '\\':
				escaped = !escaped
			case char == '"' && !inSingleQuotes:
				inDoubleQuotes = !inDoubleQuotes
			case char == '\'' && !inDoubleQuotes:
				inSingleQuotes = !inSingleQuotes
			case !inDoubleQuotes && !inSingleQuotes:
				// does a token end at this position
				if strings.HasPrefix(input[pos:], inToken) {
					splitted = append(splitted, input[tokenStart:pos+len(inToken)])
					tokenStart = pos + len(inToken)
					inToken = ""
				}
			}
		}
	}

	// end of string
	if inToken != "" {
		return splitted, fmt.Errorf("unexpected end of text, missing end token: %s", inToken)
	}
	remain := input[tokenStart:]
	if remain != "" {
		splitted = append(splitted, remain)
	}

	return splitted, nil
}

// fill empty/unused ARGx macros with empty string
func fillEmptyArgMacros(macros map[string]string) {
	for x := range 32 {
		key := fmt.Sprintf("ARG%d", x)
		if _, ok := macros[key]; !ok {
			macros[key] = ""
		}
	}
}

// replace unused macros functions so that pipes do not break naemon performance data
// ex.: %( unknownMacro | age | duration ) should not stay in the final output with pipes
func removeUnusedMacroFunctions(value string) string {
	token, err := splitToken(value, macroSplitBy)
	if err != nil {
		log.Debugf("replacing macros in %s failed: %s", value, err.Error())

		return value
	}

	var result strings.Builder
	for _, piece := range token {
		inMacro := false
		for startPattern, endPattern := range macroSplitBy {
			if !strings.HasPrefix(piece, startPattern) || !strings.HasSuffix(piece, endPattern) {
				continue
			}
			orig := piece
			piece = strings.TrimPrefix(piece, startPattern)
			piece = strings.TrimSuffix(piece, endPattern)
			piece = strings.TrimSpace(piece)

			inMacro = true
			if !strings.Contains(orig, "|") {
				result.WriteString(orig)

				break
			}
			result.WriteString(startPattern)
			result.WriteString(strings.Split(piece, "|")[0])
			result.WriteString("...")
			result.WriteString(endPattern)

			break
		}

		if !inMacro {
			result.WriteString(piece)
		}
	}

	return result.String()
}
