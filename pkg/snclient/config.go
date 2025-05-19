package snclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/humanize"
	"github.com/consol-monitoring/snclient/pkg/utils"
)

const POWERSHELL = "powershell.exe -nologo -noprofile -WindowStyle hidden -NonInteractive -ExecutionPolicy ByPass"

const (
	// DefaultPassword sets default password, login with default password is not
	// possible. It needs to be changed in the ini file.
	DefaultPassword = "CHANGEME"

	// DefaultNastyCharacters set the list of nasty characters which are not allowed
	DefaultNastyCharacters = "$|`&><'\"\\[]{}"

	// SystemCmdNastyCharacters is a list of nasty characters which are not allowed in hostnames, ex.: in check_ping
	SystemCmdNastyCharacters = "$|`&><'\"\\{}"
)

var DefaultConfig = map[string]ConfigData{
	"/modules": {
		"Logrotate":            "enabled",
		"CheckSystem":          "enabled",
		"CheckSystemUnix":      "enabled",
		"CheckAlias":           "enabled",
		"CheckExternalScripts": "enabled",
		"CheckDisk":            "enabled",
		"CheckWMI":             "disabled",
		"NRPEServer":           "disabled",
		"WEBServer":            "enabled",
		"PrometheusServer":     "disabled",
		"Updates":              "enabled",
	},
	"/settings/default": {
		"nasty characters": DefaultNastyCharacters,
	},
	"/settings/updates": {
		"channel": "stable",
	},
	"/settings/updates/channel": {
		"stable": "https://api.github.com/repos/ConSol-monitoring/snclient/releases",
		"dev":    "https://api.github.com/repos/ConSol-monitoring/snclient/actions/artifacts",
	},
	"/settings/external scripts": {
		"timeout":                "60",
		"script root":            "${scripts}", // root path of all scripts
		"script path":            "",           // load scripts from this folder automatically
		"allow arguments":        "false",
		"allow nasty characters": "false",
		"ignore perfdata":        "false",
	},
	"/settings/external scripts/wrappings": {
		"bat": `${scripts}\%SCRIPT% %ARGS%`,
		"ps1": `cmd /c echo ` +
			`If (-Not (Test-Path "${script root}\%SCRIPT%") ) ` +
			`{ Write-Host "UNKNOWN: Script ` + "`\"%SCRIPT%`\" not found.\"; exit(3) }; " +
			`${script root}\%SCRIPT% $ARGS$; ` +
			`exit($lastexitcode) | ` + POWERSHELL + ` -command -`,
		"vbs": `cscript.exe //T:30 //NoLogo "${script root}\%SCRIPT%" %ARGS%`,
	},
}

var DefaultListenTCPConfig = ConfigData{
	"allowed hosts":       "127.0.0.1, [::1]",
	"bind to":             "",
	"cache allowed hosts": "1",
	"certificate":         "${certificate-path}/server.crt",
	"certificate key":     "${certificate-path}/server.key",
	"timeout":             "30",
	"use ssl":             "0",
}

var DefaultListenHTTPConfig = ConfigData{
	"password": DefaultPassword,
}

var DefaultHTTPClientConfig = ConfigData{
	"insecure":            "false",
	"tls min version":     "tls1.2",
	"request timeout":     "60",
	"username":            "",
	"password":            "",
	"client certificates": "",
}

func init() {
	// HTTP inherits TCP defaults
	DefaultListenHTTPConfig.Merge(DefaultListenTCPConfig)
}

type ConfigFiles []string

// String returns the config files list as string.
func (c *ConfigFiles) String() string {
	return fmt.Sprintf("%s", *c)
}

// Set appends a config file to the list of config files.
func (c *ConfigFiles) Set(value string) error {
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
	recursive       bool // read includes as they appear in the config
	defaultMacros   *map[string]string
}

func NewConfig(recursive bool) *Config {
	conf := &Config{
		sections:        make(map[string]*ConfigSection, 0),
		alreadyIncluded: make(map[string]string, 0),
		recursive:       recursive,
	}

	return conf
}

func (config *Config) ToString() string {
	sortedSections := config.SectionNamesSorted()

	data := ""
	for _, name := range sortedSections {
		section := strings.TrimSpace(config.Section(name).String())
		data += section
		data += "\n\n\n"
	}

	// keep windows newlines in the ini file
	if runtime.GOOS == "windows" {
		data = strings.ReplaceAll(data, "\n", "\r\n")
	}

	return data
}

func (config *Config) WriteINI(iniPath string) error {
	// only update file if content has changed
	configData := strings.TrimSpace(config.ToString())
	currentData, err := os.ReadFile(iniPath)
	if err == nil {
		// file exists and was readable
		currentData = bytes.TrimSpace(currentData)
		if string(currentData) == configData {
			return nil
		}
	}

	file, err := os.Create(iniPath)
	if err != nil {
		return fmt.Errorf("failed to write ini %s: %s", iniPath, err.Error())
	}

	_, err = file.WriteString(configData)
	if err != nil {
		LogDebug(file.Close())

		return fmt.Errorf("failed to write ini %s: %s", iniPath, err.Error())
	}

	err = file.Close()
	if err != nil {
		return fmt.Errorf("failed to write ini %s: %s", iniPath, err.Error())
	}

	return nil
}

// ReadINI opens the config file and reads all key value pairs, separated through = and commented out with ";" and "#".
func (config *Config) ReadINI(iniPath string, snc *Agent) error {
	if prev, ok := config.alreadyIncluded[iniPath]; ok {
		return fmt.Errorf("duplicate config file found: %s, already included from %s", iniPath, prev)
	}
	config.alreadyIncluded[iniPath] = "commandline args"
	log.Tracef("stat config path: %s", iniPath)
	fileStat, err := os.Stat(iniPath)
	if err != nil {
		return fmt.Errorf("%s: %s", iniPath, err.Error())
	}

	if !fileStat.IsDir() {
		return config.ParseINIFile(iniPath, snc)
	}

	// read sub folder
	log.Debugf("recursing into config folder: %s", iniPath)
	err = filepath.WalkDir(iniPath, func(path string, dir fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%s: %s", path, err.Error())
		}
		if dir.IsDir() {
			return nil
		}
		if match, _ := filepath.Match(`*.ini`, dir.Name()); !match {
			return nil
		}

		return config.ReadINI(path, snc)
	})
	if err != nil {
		return fmt.Errorf("%s: %s", iniPath, err.Error())
	}

	return nil
}

// ParseINIFile reads ini style configuration from file path.
func (config *Config) ParseINIFile(iniPath string, snc *Agent) error {
	log.Debugf("reading config: %s", iniPath)
	data, err := os.ReadFile(iniPath)
	if err != nil {
		return fmt.Errorf("%s: %s", iniPath, err.Error())
	}
	err = config.ParseINI(string(data), iniPath, snc)
	if err != nil {
		return fmt.Errorf("config error in file %s: %s", iniPath, err.Error())
	}

	return nil
}

// ParseINI reads ini style configuration and updates config object.
// it returns the first error found but still reads the hole file
func (config *Config) ParseINI(configData, iniPath string, snc *Agent) error {
	parseErrors := []error{}
	var currentSection *ConfigSection
	currentComments := make([]string, 0)

	// trim UTF-8 BOM from start of file
	configData = strings.TrimPrefix(configData, "\xEF\xBB\xBF")

	lines := strings.Split(configData, "\n")

	// trim empty elements from the end
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	for x, line := range lines {
		lineNr := x + 1
		line = strings.TrimSpace(line)
		if line == "" || line[0] == ';' || line[0] == '#' {
			currentComments = append(currentComments, line)

			continue
		}

		// start of a new section
		if line[0] == '[' {
			// append comments to previous section unless they cuddle next section without newlines
			if currentSection != nil && len(currentComments) > 0 {
				// search comments (in reverse) for the first empty line and split those onto the next section
				for i := len(currentComments) - 1; i >= 0; i-- {
					if currentComments[i] == "" {
						currentSection.comments["_END"] = currentComments[:i]
						currentComments = currentComments[i:]

						break
					}
				}
			}
			currentBlock := strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			currentSection = config.Section(currentBlock)
			if len(currentComments) > 0 {
				currentSection.comments["_BEGIN"] = currentComments
				currentComments = make([]string, 0)
			}

			continue
		}

		if currentSection == nil {
			parseErrors = append(parseErrors, fmt.Errorf("parse error in %s:%d: found key=value pair outside of ini block", iniPath, lineNr))

			continue
		}

		// parse key and value
		val := strings.SplitN(line, "=", 2)
		if len(val) < 2 {
			parseErrors = append(parseErrors, fmt.Errorf("parse error in %s:%d: found key without '='", iniPath, lineNr))

			continue
		}
		val[0] = strings.TrimSpace(val[0])
		val[1] = strings.TrimSpace(val[1])

		// silently skip UNKNOWN values which were placeholder in nsclient
		if val[1] == "UNKNOWN" {
			continue
		}

		err := currentSection.SetRaw(val[0], val[1])
		if err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("config error in %s:%d: %s", iniPath, lineNr, err.Error()))
		}

		if len(currentComments) > 0 {
			currentSection.comments[val[0]] = currentComments
			currentComments = make([]string, 0)
		}

		// recurse directly when in an includes section to maintain order of settings
		if config.recursive && strings.HasPrefix(currentSection.name, "/includes") {
			value, err := configParseString(val[1])
			if err != nil {
				parseErrors = append(parseErrors, fmt.Errorf("%s (included in %s:%d)", err.Error(), iniPath, lineNr))

				continue
			}
			err = config.parseInclude(value, iniPath, currentSection, snc)
			if err != nil {
				parseErrors = append(parseErrors, fmt.Errorf("%s (included in %s:%d)", err.Error(), iniPath, lineNr))

				continue
			}
		}
	}

	if len(currentComments) > 0 {
		if currentSection == nil {
			currentSection = config.Section("")
		}
		currentSection.comments["_END"] = currentComments
	}

	if len(parseErrors) > 0 {
		return parseErrors[0]
	}

	return nil
}

func (config *Config) parseInclude(inclPath, srcPath string, section *ConfigSection, snc *Agent) error {
	log.Tracef("reading config include: %s", inclPath)
	if strings.HasPrefix(inclPath, "http://") || strings.HasPrefix(inclPath, "https://") {
		return config.parseHTTPInclude(inclPath, srcPath, section, snc)
	}
	if !filepath.IsAbs(inclPath) {
		inclPath = filepath.Join(filepath.Dir(srcPath), inclPath)
	}
	matchingPaths, err := filepath.Glob(inclPath)
	if err != nil {
		return fmt.Errorf("malformed include path: %s", err.Error())
	}
	if !strings.ContainsAny(inclPath, "*?") && len(matchingPaths) == 0 {
		return fmt.Errorf("no file matched: %s", inclPath)
	}

	if _, ok := config.alreadyIncluded[inclPath]; ok {
		return nil
	}

	for _, inclFile := range matchingPaths {
		err := config.ReadINI(inclFile, snc)
		if err != nil {
			return fmt.Errorf("loading included %s failed: %s", inclFile, err.Error())
		}
		config.alreadyIncluded[inclFile] = srcPath
	}

	return nil
}

func (config *Config) parseHTTPInclude(inclURL, srcPath string, section *ConfigSection, snc *Agent) error {
	sum, err := utils.Sha256Sum(inclURL)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %s", err.Error())
	}
	cacheFile := filepath.Join(os.TempDir(), fmt.Sprintf("snclient-%s.ini", sum))
	config.alreadyIncluded[inclURL] = srcPath

	// check if fetch is required (file not found, oneshotmode, ..., reload?)
	fetch := true
	exists := false
	if stat, err2 := os.Stat(cacheFile); err2 == nil {
		exists = true
		if snc.flags.Mode == ModeOneShot {
			fetch = false
		}
		// fetch if file is older than one day
		if stat.ModTime().Before(time.Now().Add(-86400 * time.Second)) {
			fetch = true
		}
	}

	if fetch {
		err = config.fetchHTTPInclude(inclURL, cacheFile, section, snc)
		if err != nil {
			if !exists {
				// fetch failed and no cache file yet is fatal
				return err
			}
			// only log a warning if file already exists and use the old one
			log.Warnf("cannot refresh http include %s: %s", inclURL, err.Error())
		}
	}

	err = config.ReadINI(cacheFile, snc)
	if err != nil {
		// if loading the config failed, but we did not refresh the file this run, remove and load fresh
		if !fetch {
			os.Remove(cacheFile)
			_ = config.fetchHTTPInclude(inclURL, cacheFile, section, snc)
			err = config.ReadINI(cacheFile, snc)
		}
		if err != nil {
			return fmt.Errorf("loading included %s (cached as %s) failed: %s", inclURL, cacheFile, err.Error())
		}
	}

	return nil
}

func (config *Config) fetchHTTPInclude(inclURL, cacheFile string, section *ConfigSection, snc *Agent) error {
	if snc == nil {
		log.Fatalf("cannot retrieve http include, got no agent")
	}
	log.Debugf("fetching config include from url: %s", inclURL)

	httpClientSection := section.Clone()
	if section.name == "/includes" {
		httpClientSection = NewConfigSection(nil, section.name)
	}
	httpClientSection.MergeData(snc.config.Section("/settings/default").data)
	httpClientSection.MergeData(DefaultHTTPClientConfig)
	httpOptions, err := snc.buildClientHTTPOptions(httpClientSection)
	if err != nil {
		return err
	}
	resp, err := snc.httpDo(context.TODO(), httpOptions, "GET", inclURL, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch include: %s -> %s", inclURL, err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch include: %s -> got status: %d", inclURL, resp.StatusCode)
	}

	// save ini to cache file
	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read http response: %s", err.Error())
	}
	// remove password before saving url
	if resp.Request.URL.User != nil {
		resp.Request.URL.User = url.UserPassword(resp.Request.URL.User.Username(), "...")
	}
	contents = append([]byte(
		fmt.Sprintf("# cached ini fetched\n# from: %s\n# date: %s\n",
			resp.Request.URL,
			time.Now().String())),
		contents...)
	err = os.WriteFile(cacheFile, contents, 0o600)
	if err != nil {
		return fmt.Errorf("failed to write cached ini to %s: %s", cacheFile, err.Error())
	}
	log.Debugf("cached ini %s written", cacheFile)

	return nil
}

// Section returns section by name or empty section.
func (config *Config) Section(name string) *ConfigSection {
	// strip square brackets
	if strings.HasPrefix(name, "[") && strings.HasSuffix(name, "]") {
		name = strings.TrimPrefix(name, "[")
		name = strings.TrimSuffix(name, "]")
	}

	if section, ok := config.sections[name]; ok {
		return section
	}

	section := NewConfigSection(config, name)
	config.sections[name] = section

	return section
}

// SectionsByPrefix returns all sections with given prefix as map.
// ex.: SectionsByPrefix("/settings/updates/channel/").
// appending trailing slash will prevent matching the parent section.
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
func configParseString(val string) (string, error) {
	val = strings.TrimSpace(val)

	switch {
	case strings.HasPrefix(val, `"`):
		count := strings.Count(val, `"`)
		switch count {
		case 1:
			return "", fmt.Errorf("unclosed quotes")
		case 2:
			if strings.HasSuffix(val, `"`) {
				val = strings.TrimPrefix(val, `"`)
				val = strings.TrimSuffix(val, `"`)
			}
		}

	case strings.HasPrefix(val, `'`):
		count := strings.Count(val, `'`)
		switch count {
		case 1:
			return "", fmt.Errorf("unclosed quotes")
		case 2:
			if strings.HasSuffix(val, `'`) {
				val = strings.TrimPrefix(val, `'`)
				val = strings.TrimSuffix(val, `'`)
			}
		}
	}

	return val, nil
}

func (config *Config) SectionNames() []string {
	keys := []string{}
	for name := range config.sections {
		keys = append(keys, name)
	}

	return keys
}

func (config *Config) SectionNamesSorted() []string {
	keys := config.SectionNames()
	ranks := map[string]int{
		"/paths":            1,
		"/modules":          5,
		"/settings/default": 10,
		"/settings":         15,
		"default":           20,
		"/includes":         50,
	}

	return utils.SortRanked(keys, ranks)
}

/* ReplaceOnDemandConfigMacros replaces any config variable macro in given string (config ini file style macros).
 * possible macros are:
 *   ${/settings/section/attribute}
 *   %(/settings/section/attribute)
 */
func (config *Config) ReplaceOnDemandConfigMacros(value string, timezone *time.Location) string {
	value = reOnDemandMacro.ReplaceAllStringFunc(value, func(macro string) string {
		orig := macro
		macro = extractMacroString(macro)

		if !strings.HasPrefix(macro, "/settings/") {
			return orig
		}

		flag := ""
		flags := strings.SplitN(macro, ":", 2)
		if len(flags) == 2 {
			macro = flags[0]
			flag = strings.ToLower(flags[1])
		}

		sectionName := path.Dir(macro)
		attribute := path.Base(macro)

		section := config.Section(sectionName)
		val, ok := section.GetString(attribute)
		if !ok {
			return orig
		}

		macroSets := map[string]string{
			macro: val,
		}

		if flag != "" {
			macro = fmt.Sprintf("%s:%s", macro, flag)
		}

		return getMacrosetsValue(macro, orig, timezone, macroSets)
	})

	return value
}

// ReplaceMacrosDefault replaces default macros in all config data.
func (config *Config) ReplaceMacrosDefault(section *ConfigSection, timezone *time.Location) {
	defaultMacros := config.DefaultMacros()
	for key, val := range section.data {
		val = ReplaceMacros(val, timezone, defaultMacros)
		section.data[key] = val

		raw := section.raw[key]
		raw = ReplaceMacros(raw, timezone, defaultMacros)
		section.raw[key] = raw
	}
}

// DefaultMacros returns a map of default macros.
// basically the /paths section and hostnames.
func (config *Config) DefaultMacros() map[string]string {
	if config.defaultMacros != nil {
		return *config.defaultMacros
	}

	defaultMacros := map[string]string{
		"hostname": "",
	}
	for key, val := range config.Section("/paths").data {
		defaultMacros[key] = val
	}

	for key, val := range GlobalMacros {
		defaultMacros[key] = val
	}

	config.defaultMacros = &defaultMacros

	hostname, err := os.Hostname()
	if err != nil {
		log.Warnf("failed to get hostname: %s", err.Error())
	}
	defaultMacros["hostname"] = hostname

	return *config.defaultMacros
}

// Reset default macros which are cached otherwise
func (config *Config) ResetDefaultMacros() {
	config.defaultMacros = nil
	config.DefaultMacros()
}

// ConfigSection contains a single config section.
type ConfigSection struct {
	cfg      *Config             // reference to parent config collection
	name     string              // section name
	data     ConfigData          // actual config data
	raw      ConfigData          // raw config data (including quotes and such)
	keys     []string            // keys from config data
	comments map[string][]string // comments sorted by config keys
}

// NewConfigSection creates a new ConfigSection.
func NewConfigSection(cfg *Config, name string) *ConfigSection {
	section := &ConfigSection{
		cfg:      cfg,
		name:     name,
		data:     make(map[string]string, 0),
		raw:      make(map[string]string, 0),
		keys:     make([]string, 0),
		comments: make(map[string][]string, 0),
	}

	return section
}

// String returns section as string
func (cs *ConfigSection) String() string {
	data := []string{}
	data = append(data, cs.comments["_BEGIN"]...)
	data = append(data, fmt.Sprintf("[%s]", cs.name))

	for _, key := range cs.keys {
		data = append(data, cs.comments[key]...)
		val := cs.data[key]
		raw := cs.raw[key]
		if raw != "" {
			val = raw
		}
		if val == "" {
			data = append(data, fmt.Sprintf("%s =", key))
		} else {
			data = append(data, fmt.Sprintf("%s = %s", key, cs.data[key]))
		}
	}

	data = append(data, cs.comments["_END"]...)

	return strings.Join(data, "\n")
}

// Set sets a raw single key/value pair. Existing keys will be overwritten.
// Raw means this value will be stored parsed (stripping quotes) but also
// be stored as is, including quotes.
func (cs *ConfigSection) SetRaw(key, value string) error {
	rawValue := value

	useAppend := false
	if strings.HasSuffix(key, "+") {
		key = strings.TrimSpace(strings.TrimSuffix(key, "+"))
		useAppend = true
	}

	value, err := configParseString(value)
	if err != nil {
		return err
	}

	if useAppend {
		curRaw, cur, ok := cs.GetStringRaw(key)
		if ok {
			value = cur + value
			rawValue = curRaw + rawValue
		}
	}

	if !cs.HasKey(key) {
		cs.keys = append(cs.keys, key)
	}

	cs.data[key] = value
	cs.raw[key] = rawValue

	return nil
}

// Set sets a single key/value pair. Existing keys will be overwritten.
func (cs *ConfigSection) Set(key, value string) {
	if !cs.HasKey(key) {
		cs.keys = append(cs.keys, key)
	}

	cs.data[key] = value
	cs.raw[key] = ""
}

// Insert is just like Set but trys to find the key in comments first and will uncomment that one
func (cs *ConfigSection) Insert(key, value string) {
	if cs.HasKey(key) {
		cs.data[key] = value

		return
	}

	// search in comments
	foundComment := false
	for name, comments := range cs.comments {
		if name == "_BEGIN" {
			continue
		}

		prefix := fmt.Sprintf("; %s =", key)
		for i, com := range comments {
			if strings.HasPrefix(com, prefix) {
				// replace with actual value
				comments[i] = fmt.Sprintf("%s = %s", key, value)
				foundComment = true

				break
			}
		}

		if foundComment {
			break
		}
	}

	if foundComment {
		// parse section back again
		tmpCfg := NewConfig(false)
		LogDebug(tmpCfg.ParseINI(cs.String(), "tmp.ini", nil))
		tmpSection := tmpCfg.Section(cs.name)
		cs.data = tmpSection.data
		cs.keys = tmpSection.keys
		cs.comments = tmpSection.comments
	} else {
		// append normally
		cs.Set(key, value)
		// migrate existing comments from the end to this option so the new option appears last
		if com, ok := cs.comments["_END"]; ok {
			cs.comments[key] = com
			delete(cs.comments, "_END")
		}
	}
}

// Remove removes a single key.
func (cs *ConfigSection) Remove(key string) {
	delete(cs.data, key)
	delete(cs.raw, key)

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
		clone.raw[k] = cs.raw[k]
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

// GetString returns string from config section.
// it returns the value if found and sets ok to true.
func (cs *ConfigSection) GetString(key string) (val string, ok bool) {
	_, val, ok = cs.GetStringRaw(key)

	return val, ok
}

// GetStringRaw returns the raw string (including quotes)
// along the clean string from config section.
// it returns the value if found and sets ok to true.
func (cs *ConfigSection) GetStringRaw(key string) (raw, val string, ok bool) {
	raw = cs.raw[key]
	val, ok = cs.data[key]
	if raw == "" {
		raw = val
	}
	if ok && cs.isUsable(key, val) {
		macros := make([]map[string]string, 0)
		if cs.cfg != nil {
			macros = append(macros, cs.cfg.DefaultMacros())
		}
		macros = append(macros, GlobalMacros)
		val = ReplaceMacros(val, nil, macros...)
		val = cs.cfg.ReplaceOnDemandConfigMacros(val, nil)

		return raw, val, ok
	}

	if cs.cfg == nil {
		return raw, val, ok
	}

	// try default folder for defaults
	base := path.Base(cs.name)
	folder := path.Dir(cs.name)
	if base != "default" && folder != "/" {
		defSection := cs.cfg.Section(folder + "/default")
		raw, val, ok = defSection.GetStringRaw(key)
		if ok {
			return raw, val, ok
		}
	}
	if folder != cs.name {
		defSection := cs.cfg.Section(folder)
		if defSection.name == cs.name {
			return raw, val, ok
		}
		raw, val, ok = defSection.GetStringRaw(key)
		if ok {
			return raw, val, ok
		}
	}
	parent := path.Dir(strings.TrimSuffix(folder, "/"))
	if parent != "." && parent != "/" && parent != "" {
		parSection := cs.cfg.Section(parent)
		raw, val, ok = parSection.GetStringRaw(key)
		if ok {
			return raw, val, ok
		}
	}

	return raw, val, ok
}

// returns true if value is usable (ex, password is not default)
func (cs *ConfigSection) isUsable(key, val string) bool {
	if key == "password" && val == DefaultPassword {
		return false
	}

	return true
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
// If value is found but cannot be parsed, error is set. Return value is in seconds.
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

// GetRegexp parses config entry as regular expression. It returns the regexp if found and sets ok to true.
// If value is found but cannot be parsed, error is set.
func (cs *ConfigSection) GetRegexp(key string) (val *regexp.Regexp, ok bool, err error) {
	raw, ok := cs.GetString(key)
	if !ok || raw == "" {
		return nil, false, nil
	}
	re, err := regexp.Compile(raw)
	if err != nil {
		return nil, true, fmt.Errorf("GetRegexp: %s", err.Error())
	}

	return re, true, nil
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

type ConfigInit []interface{}
