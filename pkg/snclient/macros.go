package snclient

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"pkg/convert"
	"pkg/humanize"
	"pkg/utils"

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

	reAsciiOnly = regexp.MustCompile(`\W`)
)

/* replaceMacros replaces variables in given string (config ini file style macros).
 * possible macros are:
 *   ${macro} / $(macro)
 *   %(macro) / %{macro}
 */
func ReplaceMacros(value string, macroSets ...map[string]string) string {
	splitBy := map[string]string{
		"$(": ")",
		"${": "}",
		"%(": ")",
		"%{": "}",
	}
	token, err := splitToken(value, splitBy)
	if err != nil {
		log.Errorf("replacing macros in %s failed: %s", value, err.Error())

		return value
	}

	var result strings.Builder
	for _, piece := range token {
		inMacro := false
		for startPattern, endPattern := range splitBy {
			if !strings.HasPrefix(piece, startPattern) || !strings.HasSuffix(piece, endPattern) {
				continue
			}
			orig := piece
			piece = strings.TrimPrefix(piece, startPattern)
			piece = strings.TrimSuffix(piece, endPattern)
			piece = strings.TrimSpace(piece)

			result.WriteString(getMacrosetsValue(piece, orig, macroSets...))
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
func ReplaceRuntimeMacros(value string, macroSets ...map[string]string) string {
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

		return getMacrosetsValue(str, orig, macroSets...)
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

func getMacrosetsValue(macro, orig string, macroSets ...map[string]string) string {
	// split by : and |
	flags := strings.FieldsFunc(macro, func(r rune) bool { return r == '|' || r == ':' })

	macro = strings.TrimSpace(flags[0])
	value := orig

	found := false
	for _, ms := range macroSets {
		if repl, ok := ms[macro]; ok {
			value = repl
			found = true

			break
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

	return (replaceMacroOperators(value, flags[1:]))
}

func replaceMacroOperators(value string, flags []string) string {
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
			value = time.Unix(convert.Int64(value), 0).Format("2006-01-02 15:04:05 MST")
		// date -> unix timestamp to utc date
		case "utc":
			value = time.Unix(convert.Int64(value), 0).UTC().Format("2006-01-02 15:04:05 MST")
		// duration -> seconds into duration string
		case "duration":
			value = utils.DurationString(time.Duration(convert.Float64(value) * float64(time.Second)))
		case "age":
			value = fmt.Sprintf("%d", time.Now().Unix()-convert.Int64(value))
		case "ascii":
			value = reAsciiOnly.ReplaceAllString(value, "")
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
				if strings.HasPrefix(input[pos:], tStart) {
					splitted = append(splitted, input[tokenStart:pos])

					inToken = tEnd
					tokenStart = pos

					break
				}
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
					inToken = ""
					tokenStart = pos + len(inToken) + 1
				}
			}
		}

		// end of string
		if pos+1 == len(input) {
			if inToken != "" {
				return splitted, fmt.Errorf("unexpected end of text, missing end token: %s", inToken)
			}
			remain := input[tokenStart:]
			if remain != "" {
				splitted = append(splitted, remain)
			}
		}
	}

	return splitted, nil
}
