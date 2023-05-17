package snclient

import (
	"strings"
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
		"script path":            "",
		"allow nasty characters": "false",
		"allow arguments":        "false",
	}

	return defaults
}

func (e *ExternalScriptsHandler) Init(snc *Agent, _ *ConfigSection, conf *Config) error {
	e.snc = snc

	for _, sectionName := range []string{"/settings/external scripts/scripts", "/settings/external scripts/wrapped scripts"} {
		scripts := conf.Section(sectionName)
		for name, command := range scripts.data {
			AvailableChecks[name] = CheckEntry{name, &CheckWrap{commandString: command}}
		}
	}

	aliases := conf.Section("/settings/external scripts/alias")
	for name, command := range aliases.data {
		f := strings.Fields(command)
		AvailableChecks[name] = CheckEntry{name, &CheckAlias{command: f[0], args: f[1:]}}
	}

	return nil
}

func (e *ExternalScriptsHandler) Start() error {
	return nil
}

func (e *ExternalScriptsHandler) Stop() {
}
