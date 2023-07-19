package snclient

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"pkg/convert"
	"pkg/humanize"
	"pkg/utils"

	"golang.org/x/exp/slices"
)

var DefaultConfig = map[string]map[string]string{
	"/modules": {
		"Logrotate":            "enabled",
		"CheckSystem":          "enabled",
		"CheckSystemUnix":      "enabled",
		"CheckExternalScripts": "enabled",
		"CheckDisk":            "enabled",
		"CheckWMI":             "disabled",
		"NRPEServer":           "disabled",
		"WEBServer":            "enabled",
		"PrometheusServer":     "disabled",
		"Updates":              "enabled",
	},
	"/settings/updates": {
		"channel": "stable",
	},
	"/settings/updates/channel": {
		"stable": "https://api.github.com/repos/ConSol-monitoring/snclient/releases",
		"dev":    "https://api.github.com/repos/ConSol-monitoring/snclient/actions/artifacts",
	},

	"/settings/external scripts/wrappings": {
		"bat": `${script root}\\%SCRIPT% %ARGS%`,
		"ps1": `cmd /c echo ` +
			`If (-Not (Test-Path "${script root}\%SCRIPT%" ) ) ` +
			`{ Write-Host "UNKNOWN: Script ${script root}\%SCRIPT% not found." ; exit(3) }; ` +
			`${script root}\%SCRIPT% $ARGS$; exit($lastexitcode) | powershell.exe -nologo -noprofile -command -`,
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
	sections        map[string]*ConfigSection
	alreadyIncluded map[string]string
}

func NewConfig() *Config {
	conf := &Config{
		sections:        make(map[string]*ConfigSection, 0),
		alreadyIncluded: make(map[string]string, 0),
	}

	return conf
}

func (config *Config) WriteINI(iniPath string) error {
	keys := []string{}
	for name := range config.sections {
		keys = append(keys, name)
	}

	ranks := map[string]int{
		"/paths":            1,
		"/modules":          5,
		"/settings/default": 10,
		"/settings":         15,
		"default":           20,
		"/includes":         50,
	}

	wordRanks := make([]WordRank, len(keys))
	for i, word := range keys {
		rank, ok := ranks[word]
		if ok {
			wordRanks[i] = WordRank{Word: word, Rank: rank}

			continue
		}

		for prefix, num := range ranks {
			if strings.HasPrefix(word, prefix) {
				if rank == 0 || rank > num {
					rank = num + 2
				}
			}
		}
		if rank != 0 {
			wordRanks[i] = WordRank{Word: word, Rank: rank}
		}

		wordRanks[i] = WordRank{Word: word, Rank: ranks["default"]}
	}

	sort.Sort(ByRank(wordRanks))

	data := ""
	for _, name := range wordRanks {
		section := config.Section(name.Word)
		data += section.String()
		data += "\n\n"
	}

	file, err := os.Create(iniPath)
	if err != nil {
		return fmt.Errorf("failed to write ini %s: %s", iniPath, err.Error())
	}
	defer file.Close()

	_, err = file.WriteString(data)
	if err != nil {
		return fmt.Errorf("failed to write ini %s: %s", iniPath, err.Error())
	}

	return nil
}

// ReadINI opens the config file and reads all key value pairs, separated through = and commented out with ";" and "#".
func (config *Config) ReadINI(iniPath string) error {
	if prev, ok := config.alreadyIncluded[iniPath]; ok {
		return fmt.Errorf("duplicate config file found: %s, already included from %s", iniPath, prev)
	}
	config.alreadyIncluded[iniPath] = "command args"
	log.Tracef("stat config path: %s", iniPath)
	file, err := os.Open(iniPath)
	if err != nil {
		return fmt.Errorf("%s: %s", iniPath, err.Error())
	}
	fileStat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("%s: %s", iniPath, err.Error())
	}
	if fileStat.IsDir() {
		log.Debugf("recursing into config folder: %s", iniPath)
		err := filepath.WalkDir(iniPath, func(path string, dir fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("%s: %s", path, err.Error())
			}
			if dir.IsDir() {
				return nil
			}
			if match, _ := filepath.Match(`*.ini`, dir.Name()); !match {
				return nil
			}

			return config.ReadINI(path)
		})
		if err != nil {
			return fmt.Errorf("%s: %s", iniPath, err.Error())
		}

		return nil
	}

	log.Debugf("reading config: %s", iniPath)
	err = config.ParseINI(file, iniPath, true)
	if err != nil {
		return fmt.Errorf("config error in file %s: %s", iniPath, err.Error())
	}

	return nil
}

func (config *Config) ParseINI(file io.Reader, iniPath string, recursive bool) error {
	var currentSection *ConfigSection
	lineNr := 0

	scanner := bufio.NewScanner(file)
	currentComments := make([]string, 0)
	for scanner.Scan() {
		lineNr++

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if line[0] == ';' || line[0] == '#' {
			line = strings.TrimSpace(line[1:])
			currentComments = append(currentComments, line)

			continue
		}

		if line[0] == '[' {
			currentBlock := strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			currentSection = config.Section(currentBlock)
			if len(currentComments) > 0 {
				currentSection.comments[""] = currentComments
				currentComments = make([]string, 0)
			}

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

		currentSection.Set(val[0], value)
		if len(currentComments) > 0 {
			currentSection.comments[val[0]] = currentComments
			currentComments = make([]string, 0)
		}

		// recurse directly when in an includes section to maintain order of settings
		if recursive && currentSection.name == "/includes" {
			err := config.parseInclude(value, iniPath)
			if err != nil {
				return fmt.Errorf("%s (included in %s:%d)", err.Error(), iniPath, lineNr)
			}
		}
	}

	return nil
}

func (config *Config) parseInclude(inclPath, srcPath string) error {
	log.Tracef("reading config include: %s", inclPath)
	matchingPaths, err := filepath.Glob(inclPath)
	if err != nil {
		return fmt.Errorf("malformed include path: %s", err.Error())
	}

	if _, ok := config.alreadyIncluded[inclPath]; ok {
		return nil
	}

	for _, inclFile := range matchingPaths {
		err := config.ReadINI(inclFile)
		if err != nil {
			return fmt.Errorf("included readini failed: %s", err.Error())
		}
		config.alreadyIncluded[inclFile] = srcPath
	}

	return nil
}

// Section returns section by name or empty section.
func (config *Config) Section(name string) *ConfigSection {
	if section, ok := config.sections[name]; ok {
		return section
	}

	section := NewConfigSection(config, name)
	config.sections[name] = section

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
	cfg      *Config
	name     string
	data     ConfigData
	keys     []string
	comments map[string][]string
}

// NewConfigSection creates a new ConfigSection.
func NewConfigSection(cfg *Config, name string) *ConfigSection {
	section := &ConfigSection{
		cfg:      cfg,
		name:     name,
		data:     make(map[string]string, 0),
		keys:     make([]string, 0),
		comments: make(map[string][]string, 0),
	}

	return section
}

// String returns section as string
func (cs *ConfigSection) String() string {
	data := []string{}
	for _, comment := range cs.comments[""] {
		data = append(data, fmt.Sprintf("; %s", comment))
	}
	data = append(data, fmt.Sprintf("[%s]", cs.name))

	for _, key := range cs.keys {
		for _, comment := range cs.comments[key] {
			data = append(data, fmt.Sprintf("; %s", comment))
		}
		data = append(data, fmt.Sprintf("%s = %s", key, cs.data[key]))
	}

	return strings.Join(data, "\n")
}

// Set sets a single key/value pair. Existing keys will be overwritten.
func (cs *ConfigSection) Set(key, value string) {
	cs.data[key] = value
	if !slices.Contains(cs.keys, key) {
		cs.keys = append(cs.keys, key)
	}
}

// Remove removes a single key.
func (cs *ConfigSection) Remove(key string) {
	delete(cs.data, key)

	index := slices.Index(cs.keys, key)
	if index != -1 {
		cs.keys = slices.Delete(cs.keys, index, index+1)
	}
}

// Merge merges defaults into ConfigSection.
// (first value wins, later ones will be discarded)
func (cs *ConfigSection) MergeSection(defaults *ConfigSection) {
	cs.MergeData(defaults.data)
}

// MergeSections merges multiple defaults into ConfigSection.
// (first value wins, later ones will be discarded)
func (cs *ConfigSection) MergeSections(defaults ...*ConfigSection) {
	for _, def := range defaults {
		cs.MergeSection(def)
	}
}

// MergeData merges config maps into a section
func (cs *ConfigSection) MergeData(defaults ConfigData) {
	for key, val := range defaults {
		if !cs.HasKey(key) {
			cs.Set(key, val)
		}
	}
}

// Clone creates a copy.
func (cs *ConfigSection) Clone() *ConfigSection {
	clone := NewConfigSection(cs.cfg, cs.name)
	for k, v := range cs.data {
		clone.data[k] = v
	}
	clone.keys = append(clone.keys, clone.keys...)
	clone.cfg = cs.cfg
	clone.name = cs.name

	return clone
}

// Keys returns list of config keys.
func (cs *ConfigSection) Keys() []string {
	return cs.keys
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
		// treat literal UNKNOWN as empty strings, they are the default value in nsclient.ini
		if val == "UNKNOWN" {
			val = ""
		}
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

// Merge merges two config maps (unordered)
func (d *ConfigData) Merge(defaults ConfigData) {
	for key, value := range defaults {
		if _, ok := (*d)[key]; !ok {
			(*d)[key] = value
		}
	}
}

type WordRank struct {
	Word string
	Rank int
}
type ByRank []WordRank

func (a ByRank) Len() int           { return len(a) }
func (a ByRank) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByRank) Less(i, j int) bool { return a[i].Rank < a[j].Rank || a[i].Word < a[j].Word }
