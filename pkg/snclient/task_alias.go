package snclient

import (
	"fmt"
	"path"

	"github.com/consol-monitoring/snclient/pkg/utils"
)

func init() {
	RegisterModule(&AvailableTasks, "CheckAlias", "/settings/external scripts/alias", NewAliasHandler)
}

type AliasHandler struct {
	noCopy noCopy
}

func NewAliasHandler() Module {
	return &AliasHandler{}
}

func (a *AliasHandler) Defaults(_ *AgentRunSet) ConfigData {
	return nil
}

func (a *AliasHandler) Init(_ *Agent, section *ConfigSection, conf *Config, runSet *AgentRunSet) error {
	// merge alias shortcuts into separate config sections
	for name, command := range section.data {
		cmdConf := conf.Section("/settings/external scripts/alias/" + name)
		if !cmdConf.HasKey("command") {
			cmdConf.Set("command", command)
		}
	}

	// now read all aliases into available checks
	for sectionName := range conf.SectionsByPrefix("/settings/external scripts/alias/") {
		name := path.Base(sectionName)
		if name == "default" {
			continue
		}
		cmdConf := conf.Section(sectionName)
		if command, ok := cmdConf.GetString("command"); ok {
			f := utils.Tokenize(command)
			log.Tracef("registered alias script: %s -> %s", name, command)
			runSet.cmdAliases[name] = CheckEntry{name, func() CheckHandler { return &CheckAlias{command: f[0], args: f[1:], config: cmdConf} }}
		} else {
			return fmt.Errorf("missing command in alias script %s", name)
		}
	}
	log.Tracef("aliases initialized")

	return nil
}

func (a *AliasHandler) Start() error {
	return nil
}

func (a *AliasHandler) Stop() {
}
