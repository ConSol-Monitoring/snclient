package snclient

import (
	"fmt"
	"regexp"
	"strings"

	"pkg/convert"
	"pkg/utils"
)

// PerfConfig contains a single perf-config item.
type PerfConfig struct {
	Selector string
	regex    *regexp.Regexp
	Ignore   bool
	Prefix   string
	Suffix   string
	Unit     string
	Magic    float64
}

func NewPerfConfig(raw string) ([]PerfConfig, error) {
	list := []PerfConfig{}

	token := utils.TokenizeBy(strings.TrimSpace(raw), "()", false, true)

	for len(token) > 0 {
		if len(token) < 4 {
			return nil, fmt.Errorf("unexpected end of perf-config, remaining token: %#v", token)
		}

		selector, err := utils.TrimQuotes(strings.TrimSpace(token[0]))
		if err != nil {
			return nil, fmt.Errorf("quotes error in perf-config in '%s': %s", token[0], err.Error())
		}
		rawConf := strings.TrimSpace(token[2])
		if token[1] != "(" {
			return nil, fmt.Errorf("expected opening bracket in perf-config after '%s'", token[0])
		}
		if token[3] != ")" {
			return nil, fmt.Errorf("expected closing bracket in perf-config after '%s'", token[2])
		}

		perf := PerfConfig{
			Selector: selector,
		}
		err = perf.parseArgs(rawConf)
		if err != nil {
			return nil, fmt.Errorf("parse error in perf-config args in '%s': %s", rawConf, err.Error())
		}

		if strings.Contains(perf.Selector, "*") {
			patternText := perf.Selector
			patternText = strings.ReplaceAll(patternText, "*", "WILD_CARD_ASTERISK")
			patternText = regexp.QuoteMeta(patternText)
			patternText = strings.ReplaceAll(patternText, "WILD_CARD_ASTERISK", ".*")
			re, err := regexp.Compile(patternText)
			if err != nil {
				return nil, fmt.Errorf("failed to convert pattern '%s' into regexp: %s", patternText, err.Error())
			}
			perf.regex = re
		}

		list = append(list, perf)

		token = token[4:]
	}

	return list, nil
}

// Match returns true if given string matches the selector
func (p *PerfConfig) Match(name string) bool {
	if p.regex != nil {
		return p.regex.MatchString(name)
	}

	return strings.Contains(name, p.Selector)
}

func (p *PerfConfig) parseArgs(raw string) error {
	conf := utils.TokenizeBy(raw, ";", false, false)
	p.Magic = 1
	for _, confItem := range conf {
		confItem = strings.TrimSpace(confItem)
		keyVal := strings.SplitN(confItem, ":", 2)
		if len(keyVal) != 2 {
			return fmt.Errorf("syntax error (key:value) in perf-config, expected colon in '%s'", raw)
		}
		rawVal, err := utils.TrimQuotes(strings.TrimSpace(keyVal[1]))
		if err != nil {
			return fmt.Errorf("quotes error in perf-config in '%s': %s", confItem, err.Error())
		}
		switch strings.ToLower(keyVal[0]) {
		case "unit":
			p.Unit = rawVal
		case "suffix":
			p.Suffix = rawVal
		case "prefix":
			p.Prefix = rawVal
		case "ignored", "ignore":
			ign, err := convert.BoolE(rawVal)
			if err != nil {
				return fmt.Errorf("parse error in perf-config in '%s': %s", confItem, err.Error())
			}
			p.Ignore = ign
		case "magic":
			magic, err := convert.Float64E(rawVal)
			if err != nil {
				return fmt.Errorf("failed to parse magic number in perf-config in '%s': %s", confItem, err.Error())
			}
			p.Magic = magic
		default:
			return fmt.Errorf("unknown attribute %s in perf-config in '%s'", keyVal[0], raw)
		}
	}

	return nil
}
