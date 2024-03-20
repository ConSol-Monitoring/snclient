package snclient

import (
	"path"
)

func init() {
	RegisterModule(&AvailableTasks, "ManagedExporterServer", "/settings/ManagedExporter", NewManagedExporterHandler)
}

type ManagedExporterHandler struct {
	noCopy noCopy
	snc    *Agent
}

func NewManagedExporterHandler() Module {
	return &ManagedExporterHandler{}
}

func (ch *ManagedExporterHandler) Defaults(_ *AgentRunSet) ConfigData {
	defaults := ConfigData{}

	return defaults
}

func (ch *ManagedExporterHandler) Init(snc *Agent, _ *ConfigSection, conf *Config, _ *AgentRunSet) error {
	ch.snc = snc

	nr := ch.registerHandler(conf)
	log.Tracef("%d managed exporter(s) initialized", nr)

	return nil
}

func (ch *ManagedExporterHandler) Start() error {
	return nil
}

func (ch *ManagedExporterHandler) Stop() {
}

func (ch *ManagedExporterHandler) registerHandler(conf *Config) (nr int64) {
	// read all available exporters
	for sectionName := range conf.SectionsByPrefix("/settings/ManagedExporter/") {
		name := path.Base(sectionName)
		if name == "default" {
			continue
		}
		expModule := func() Module {
			expModule := &HandlerManagedExporter{
				name: name,
			}

			return expModule
		}
		RegisterModule(&AvailableListeners, "ManagedExporterServer", sectionName, expModule)
		nr++
	}

	return nr
}
