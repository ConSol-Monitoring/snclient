package snclient

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"pkg/utils"
)

func init() {
	RegisterModule(&AvailableTasks, "CheckExternalScripts", "/settings/external scripts", NewExternalScriptsHandler)
}

type ExternalScriptsHandler struct {
	noCopy noCopy
	snc    *Agent
}

func NewExternalScriptsHandler() Module {
	return &ExternalScriptsHandler{}
}

func (e *ExternalScriptsHandler) Defaults() ConfigData {
	defaults := ConfigData{
		"timeout":                "60",
		"script root":            "${scripts}", // root path of all scripts
		"script path":            "",           // load scripts from this folder automatically
		"allow nasty characters": "false",
		"allow arguments":        "false",
		"ignore perfdata":        "false",
	}

	return defaults
}

func (e *ExternalScriptsHandler) Init(snc *Agent, defaultScriptConfig *ConfigSection, conf *Config, _ *ModuleSet) error {
	e.snc = snc

	if err := e.registerScriptPath(defaultScriptConfig, conf); err != nil {
		return err
	}
	if err := e.registerScripts(conf); err != nil {
		return err
	}
	if err := e.registerWrapped(conf); err != nil {
		return err
	}
	if err := e.registerAliases(conf); err != nil {
		return err
	}

	log.Tracef("external scripts initialized")

	return nil
}

func (e *ExternalScriptsHandler) Start() error {
	return nil
}

func (e *ExternalScriptsHandler) Stop() {
}

func (e *ExternalScriptsHandler) registerScripts(conf *Config) error {
	// merge command shortcuts into separate config sections
	scripts := conf.Section("/settings/external scripts/scripts")
	for name, command := range scripts.data {
		cmdConf := conf.Section("/settings/external scripts/scripts/" + name)
		if !cmdConf.HasKey("command") {
			cmdConf.Set("command", command)
		}
	}

	// now read all scripts into available checks
	for sectionName := range conf.SectionsByPrefix("/settings/external scripts/scripts/") {
		name := path.Base(sectionName)
		if name == "default" {
			continue
		}
		cmdConf := conf.Section(sectionName)
		if command, ok := cmdConf.GetString("command"); ok {
			log.Tracef("registered script: %s -> %s", name, command)
			AvailableChecks[name] = CheckEntry{name, func() CheckHandler { return &CheckWrap{name: name, commandString: command, config: cmdConf} }}
		} else {
			return fmt.Errorf("missing command in external script %s", name)
		}
	}

	return nil
}

func (e *ExternalScriptsHandler) registerWrapped(conf *Config) error {
	// merge wrapped command shortcuts into separate config sections
	scripts := conf.Section("/settings/external scripts/wrapped scripts")
	for name, command := range scripts.data {
		cmdConf := conf.Section("/settings/external scripts/wrapped scripts/" + name)
		if !cmdConf.HasKey("command") {
			cmdConf.Set("command", command)
		}
	}

	// now read all wrapped scripts into available checks
	for sectionName := range conf.SectionsByPrefix("/settings/external scripts/wrapped scripts/") {
		name := path.Base(sectionName)
		if name == "default" {
			continue
		}
		cmdConf := conf.Section(sectionName)
		if command, ok := cmdConf.GetString("command"); ok {
			log.Tracef("registered wrapped script: %s -> %s", name, command)
			AvailableChecks[name] = CheckEntry{name, func() CheckHandler {
				return &CheckWrap{name: name, commandString: command, wrapped: true, config: cmdConf}
			}}
		} else {
			return fmt.Errorf("missing command in wrapped external script %s", name)
		}
	}

	return nil
}

func (e *ExternalScriptsHandler) registerAliases(conf *Config) error {
	// merge alias shortcuts into separate config sections
	scripts := conf.Section("/settings/external scripts/alias")
	for name, command := range scripts.data {
		cmdConf := conf.Section("/settings/external scripts/alias/" + name)
		if !cmdConf.HasKey("command") {
			cmdConf.Set("command", command)
		}
	}

	// now read all alias into available checks
	for sectionName := range conf.SectionsByPrefix("/settings/external scripts/alias/") {
		name := path.Base(sectionName)
		if name == "default" {
			continue
		}
		cmdConf := conf.Section(sectionName)
		if command, ok := cmdConf.GetString("command"); ok {
			f := utils.Tokenize(command)
			log.Tracef("registered wrapped script: %s -> %s", name, command)
			AvailableChecks[name] = CheckEntry{name, func() CheckHandler { return &CheckAlias{command: f[0], args: f[1:], config: cmdConf} }}
		} else {
			return fmt.Errorf("missing command in alias script %s", name)
		}
	}

	return nil
}

func (e *ExternalScriptsHandler) registerScriptPath(defaultScriptConfig *ConfigSection, conf *Config) error {
	scriptPath, ok := defaultScriptConfig.GetString("script path")
	if !ok || scriptPath == "" {
		return nil
	}

	_, err := os.Stat(scriptPath)
	if os.IsNotExist(err) {
		log.Warnf("script path %s: folder does not exist", scriptPath)

		return nil
	}

	pattern := filepath.Join(scriptPath, "*.*")
	log.Debugf("script path: loading all scripts matching: %s", pattern)
	scripts, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to list script path: %s", err.Error())
	}

	for _, command := range scripts {
		name := filepath.Base(command)
		cmdConf := conf.Section("/settings/external scripts/scripts/" + name)
		if !cmdConf.HasKey("command") {
			allow, _, _ := defaultScriptConfig.GetBool("allow arguments")
			if allow {
				cmdConf.Set("command", command+" %ARGS%")
			} else {
				cmdConf.Set("command", command)
			}
		}
	}

	return nil
}
