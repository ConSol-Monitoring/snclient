package snclient

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var reMacro = regexp.MustCompile(`\$\{\s*[a-zA-Z\-_]+\s*\}`)

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
	sections map[string]ConfigSection
}

func NewConfig() *Config {
	conf := &Config{
		sections: make(map[string]ConfigSection, 0),
	}

	return conf
}

// opens the config file and reads all key value pairs, separated through = and commented out with ";".
func (config *Config) ReadSettingsFile(path string) error {
	log.Debugf("reading config: %s", path)
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot read file %s: %s", path, err.Error())
	}

	currentBlock := ""
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
			currentBlock = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			currentSection = config.Section(currentBlock)

			continue
		}

		if currentBlock == "" {
			return fmt.Errorf("parse error in %s:%d: found key=value outside of block", path, lineNr)
		}

		// get both values
		val := strings.SplitN(line, "=", 2)
		if len(val) < 2 {
			return fmt.Errorf("parse error in %s:%d: found key without '='", path, lineNr)
		}
		val[0] = strings.TrimSpace(val[0])
		val[1] = strings.TrimSpace(val[1])

		currentSection.Set(val[0], val[1])
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

// ReplaceMacros replaces variables in given string.
func (config *Config) ReplaceMacros(value string) string {
	value = reMacro.ReplaceAllStringFunc(value, func(str string) string {
		orig := str
		str = strings.TrimPrefix(str, "${")
		str = strings.TrimSuffix(str, "}")
		str = strings.TrimSpace(str)
		repl, ok, err := config.Section("/paths").GetString(str)
		switch {
		case err != nil:
			log.Warnf("cannot expand macro: ${%s}", str, err.Error())

			return orig
		case !ok:
			log.Warnf("using undefined macro: ${%s}", str)

			return orig
		}

		return repl
	})

	return value
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

// ConfigData contains data for a section.
type ConfigData map[string]string

// Merge merges defaults into ConfigSection.
func (d *ConfigData) Merge(defaults ConfigData) {
	for key, value := range defaults {
		if _, ok := (*d)[key]; !ok {
			(*d)[key] = value
		}
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

// GetString parses string from config section, it returns the value if found and sets ok to true.
// If value is found but cannot be parsed, error is set.
func (cs *ConfigSection) GetString(key string) (val string, ok bool, err error) {
	val, ok = cs.data[key]
	if !ok {
		return "", false, nil
	}
	val = strings.TrimSpace(val)
	val = cs.cfg.ReplaceMacros(val)

	switch {
	case strings.HasPrefix(val, `"`):
		if !strings.HasSuffix(val, `"`) {
			return "", true, fmt.Errorf("unclosed quotes in %s: ", val)
		}
		val = strings.TrimPrefix(val, `"`)
		val = strings.TrimSuffix(val, `"`)

	case strings.HasPrefix(val, `'`):
		if !strings.HasSuffix(val, `'`) {
			return "", true, fmt.Errorf("unclosed quotes in %s: ", val)
		}
		val = strings.TrimPrefix(val, `'`)
		val = strings.TrimSuffix(val, `'`)
	}

	return val, true, nil
}

// GetInt parses int64 from config section, it returns the value if found and sets ok to true.
// If value is found but cannot be parsed, error is set.
func (cs *ConfigSection) GetInt(key string) (num int64, ok bool, err error) {
	val, ok, err := cs.GetString(key)
	if err != nil {
		return 0, ok, fmt.Errorf("ParseInt: %s", err.Error())
	}
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
	raw, ok, err := cs.GetString(key)
	if err != nil {
		return false, ok, fmt.Errorf("cannot parse bool: %s", err.Error())
	}
	if !ok {
		return false, false, nil
	}
	switch strings.ToLower(raw) {
	case "1", "enabled", "true":
		return true, true, nil
	case "0", "disabled", "false":
		return false, true, nil
	}

	return false, true, fmt.Errorf("cannot parse boolean value from %s", raw)
}

func parseTLSMinVersion(version string) (uint16, error) {
	switch strings.ToLower(version) {
	case "":
		return 0, nil
	case "tls10", "tls1.0":
		return tls.VersionTLS10, nil
	case "tls11", "tls1.1":
		return tls.VersionTLS11, nil
	case "tls12", "tls1.2":
		return tls.VersionTLS12, nil
	case "tls13", "tls1.3":
		return tls.VersionTLS13, nil
	default:
		err := fmt.Errorf("cannot parse %s into tls version, supported values are: tls1.0, tls1.1, tls1.2, tls1.3", version)

		return 0, err
	}
}
