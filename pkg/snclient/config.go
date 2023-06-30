package snclient

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"

	"pkg/convert"
	"pkg/humanize"
	"pkg/utils"
)

var DefaultConfig = map[string]*ConfigData{
	"/modules": {
		"Logrotate":            "enabled",
		"CheckSystem":          "enabled",
		"CheckSystemUnix":      "enabled",
		"CheckExternalScripts": "enabled",
		"Updates":              "enabled",
	},
	"/settings/updates": {
		"channel": "stable",
	},
	"/settings/updates/channel": {
		"stable": "https://api.github.com/repos/ConSol-monitoring/snclient/releases",
		"dev":    "https://api.github.com/repos/ConSol-monitoring/snclient/actions/artifacts",
	},
}

type configFiles []string

// String returns the config files list as string.
func (c *configFiles) String() string {
	return fmt.Sprintf("%s", *c)
}

// Set appends a config file to the list of config files.
func (c *configFiles) Set(value string) error {
	// check if the file exists but skip errors for file globs
	_, err := os.Stat(value)
	if err != nil && !strings.ContainsAny(value, "?*") {
		return fmt.Errorf("failed to read config file: %s", err.Error())
	}

	*c = append(*c, value)

	return nil
}

// Config contains the merged config over all config files.
type Config struct {
	sections        map[string]ConfigSection
	alreadyIncluded map[string]string
}

func NewConfig() *Config {
	conf := &Config{
		sections:        make(map[string]ConfigSection, 0),
		alreadyIncluded: make(map[string]string, 0),
	}

	return conf
}

// ReadINI opens the config file and reads all key value pairs, separated through = and commented out with ";" and "#".
func (config *Config) ReadINI(iniPath string) error {
	if prev, ok := config.alreadyIncluded[iniPath]; ok {
		return fmt.Errorf("duplicate config file found: %s, already included from %s", iniPath, prev)
	}
	config.alreadyIncluded[iniPath] = "command args"
	log.Debugf("reading config: %s", iniPath)
	file, err := os.Open(iniPath)
	if err != nil {
		return fmt.Errorf("%s: %s", iniPath, err.Error())
	}

	err = config.parseINI(file, iniPath)
	if err != nil {
		return fmt.Errorf("config error in file %s: %s", iniPath, err.Error())
	}

	// import includes
	inclSection := config.Section("/includes")
	for name, incl := range inclSection.data {
		log.Tracef("reading config include: %s", incl)
		delete(inclSection.data, name)
		if _, ok := config.alreadyIncluded[incl]; !ok {
			err := config.ReadINI(incl)
			if err != nil {
				return fmt.Errorf("readini failed: %s", err.Error())
			}
			config.alreadyIncluded[incl] = iniPath
		}
	}

	return nil
}

func (config *Config) parseINI(file io.Reader, iniPath string) error {
	var currentSection *ConfigSection
	lineNr := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineNr++

		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == ';' || line[0] == '#' {
			continue
		}

		if line[0] == '[' {
			currentBlock := strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			currentSection = config.Section(currentBlock)

			continue
		}

		if currentSection == nil {
			return fmt.Errorf("parse error in %s:%d: found key=value pair outside of ini block", iniPath, lineNr)
		}

		// get both values
		val := strings.SplitN(line, "=", 2)
		if len(val) < 2 {
			return fmt.Errorf("parse error in %s:%d: found key without '='", iniPath, lineNr)
		}
		val[0] = strings.TrimSpace(val[0])
		val[1] = strings.TrimSpace(val[1])

		value, err := config.parseString(val[1])
		if err != nil {
			return fmt.Errorf("config error in %s:%d: %s", iniPath, lineNr, err.Error())
		}

		// on duplicate entries the first one wins
		if _, ok := currentSection.data[val[0]]; ok {
			log.Warnf("tried to redefine %s/%s in %s:%d", currentSection.name, val[0], iniPath, lineNr)
		} else {
			currentSection.Set(val[0], value)
		}
	}

	return nil
}

// Section returns section by name or empty section.
func (config *Config) Section(name string) *ConfigSection {
	if section, ok := config.sections[name]; ok {
		return &section
	}

	section := NewConfigSection(config, name)
	config.sections[name] = *section

	return section
}

// SectionsByPrefix returns section by name or empty section.
func (config *Config) SectionsByPrefix(prefix string) map[string]*ConfigSection {
	list := make(map[string]*ConfigSection, 0)
	for name := range config.sections {
		if strings.HasPrefix(name, prefix) {
			list[name] = config.Section(name)
		}
	}

	return list
}

// parseString parses string from config section.
func (config *Config) parseString(val string) (string, error) {
	val = strings.TrimSpace(val)

	switch {
	case strings.HasPrefix(val, `"`):
		if !strings.HasSuffix(val, `"`) {
			return "", fmt.Errorf("unclosed quotes")
		}
		val = strings.TrimPrefix(val, `"`)
		val = strings.TrimSuffix(val, `"`)

	case strings.HasPrefix(val, `'`):
		if !strings.HasSuffix(val, `'`) {
			return "", fmt.Errorf("unclosed quotes")
		}
		val = strings.TrimPrefix(val, `'`)
		val = strings.TrimSuffix(val, `'`)
	}

	return val, nil
}

// ConfigSection contains a single config section.
type ConfigSection struct {
	cfg  *Config
	name string
	data ConfigData
}

// NewConfigSection creates a new ConfigSection.
func NewConfigSection(cfg *Config, name string) *ConfigSection {
	section := &ConfigSection{
		cfg:  cfg,
		name: name,
		data: make(map[string]string, 0),
	}

	return section
}

// Set sets a single key/value pair. Existing keys will be overwritten.
func (cs *ConfigSection) Set(key, value string) {
	cs.data[key] = value
}

// Merge merges defaults into ConfigSection.
func (cs *ConfigSection) Merge(defaults ConfigSection) {
	cs.data.Merge(defaults.data)
}

// MergeDefaults merges multiple defaults into ConfigSection.
func (cs *ConfigSection) MergeDefaults(defaults ...*ConfigSection) {
	for _, def := range defaults {
		cs.data.Merge(def.data)
	}
}

// Clone creates a copy.
func (cs *ConfigSection) Clone() *ConfigSection {
	clone := NewConfigSection(cs.cfg, cs.name)
	for k, v := range cs.data {
		clone.data[k] = v
	}

	return clone
}

// HasKey returns true if given key exists in this config section
func (cs *ConfigSection) HasKey(key string) (ok bool) {
	_, ok = cs.data[key]

	return ok
}

// GetString parses string from config section, it returns the value if found and sets ok to true.
func (cs *ConfigSection) GetString(key string) (val string, ok bool) {
	val, ok = cs.data[key]
	if ok {
		macros := make([]map[string]string, 0)
		if cs.cfg != nil {
			macros = append(macros, cs.cfg.Section("/paths").data)
		}
		macros = append(macros, GlobalMacros)
		val = ReplaceMacros(val, macros...)

		return val, ok
	}

	if cs.cfg == nil {
		return val, ok
	}

	// try default folder for defaults
	base := path.Base(cs.name)
	folder := path.Dir(cs.name)
	if base != "default" && folder != "/" {
		defSection := cs.cfg.Section(folder + "/default")
		val, ok := defSection.GetString(key)
		if ok {
			return val, ok
		}
	}
	if folder != cs.name {
		defSection := cs.cfg.Section(folder)
		val, ok := defSection.GetString(key)
		if ok {
			return val, ok
		}
	}
	parent := path.Dir(strings.TrimSuffix(folder, "/"))
	if parent != "." && parent != "/" && parent != "" {
		parSection := cs.cfg.Section(parent)
		val, ok := parSection.GetString(key)
		if ok {
			return val, ok
		}
	}

	return val, ok
}

// GetInt parses int64 from config section, it returns the value if found and sets ok to true.
// If value is found but cannot be parsed, error is set.
func (cs *ConfigSection) GetInt(key string) (num int64, ok bool, err error) {
	val, ok := cs.GetString(key)
	if !ok {
		return 0, false, nil
	}
	num, err = strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, true, fmt.Errorf("ParseInt: %s", err.Error())
	}

	return num, true, nil
}

// GetBool parses bool from config section, it returns the value if found and sets ok to true.
// If value is found but cannot be parsed, error is set.
func (cs *ConfigSection) GetBool(key string) (val, ok bool, err error) {
	raw, ok := cs.GetString(key)
	if !ok {
		return false, false, nil
	}
	val, err = convert.BoolE(raw)
	if err != nil {
		return false, true, fmt.Errorf("parseBool %s: %s", raw, err.Error())
	}

	return val, ok, nil
}

// GetDuration parses duration value from config section, it returns the value if found and sets ok to true.
// If value is found but cannot be parsed, error is set.
func (cs *ConfigSection) GetDuration(key string) (val float64, ok bool, err error) {
	raw, ok := cs.GetString(key)
	if !ok {
		return 0, false, nil
	}
	num, err := utils.ExpandDuration(raw)
	if err != nil {
		return 0, true, fmt.Errorf("GetDuration: %s", err.Error())
	}

	return num, true, nil
}

// GetBytes parses int value with optional SI
// If value is found but cannot be parsed, error is set.
func (cs *ConfigSection) GetBytes(key string) (val uint64, ok bool, err error) {
	raw, ok := cs.GetString(key)
	if !ok {
		return 0, false, nil
	}
	num, err := humanize.ParseBytes(raw)
	if err != nil {
		return 0, true, fmt.Errorf("GetBytes: %s", err.Error())
	}

	return num, true, nil
}

// ConfigData contains data for a section.
type ConfigData map[string]string

// Keys returns all config keys
func (d *ConfigData) Keys() []string {
	keys := make([]string, 0, len(*d))
	for k := range *d {
		keys = append(keys, k)
	}

	return keys
}

// Merge merges defaults into ConfigData.
func (d *ConfigData) Merge(defaults ConfigData) {
	for key, value := range defaults {
		if _, ok := (*d)[key]; !ok {
			(*d)[key] = value
		}
	}
}
