package snclient

import (
	"fmt"
	"path"

	"github.com/consol-monitoring/snclient/pkg/utils"
)

func init() {
	RegisterModule(&AvailableTasks, "CheckAlias", "/settings/external scripts/alias", NewAliasHandler, nil)
}

type AliasHandler struct {
	noCopy noCopy
}

func NewAliasHandler() Module {
	return &AliasHandler{}
}

func (a *AliasHandler) Init(_ *Agent, section *ConfigSection, conf *Config, runSet *AgentRunSet) error {
	// merge alias shortcuts into separate config sections
	for name := range section.data {
		cmdConf := conf.Section("/settings/external scripts/alias/" + name)
		if !cmdConf.HasKey("command") {
			raw, _, _ := section.GetStringRaw(name)
			cmdConf.Set("command", raw)
		}
	}

	// now read all aliases into available checks
	for sectionName := range conf.SectionsByPrefix("/settings/external scripts/alias/") {
		name := path.Base(sectionName)
		if name == "default" {
			continue
		}
		cmdConf := conf.Section(sectionName)
		if command, _, ok := cmdConf.GetStringRaw("command"); ok {
			args := utils.Tokenize(command)
			args, err := utils.TrimQuotesList(args)
			if err != nil {
				return fmt.Errorf("failed to register alias %s: %s", name, err.Error())
			}
			log.Tracef("registered alias script: %s -> %s", name, command)
			runSet.cmdAliases[name] = CheckEntry{name, func() CheckHandler { return &CheckAlias{command: args[0], args: args[1:], config: cmdConf} }}
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
