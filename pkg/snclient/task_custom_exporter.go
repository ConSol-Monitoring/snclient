package snclient

import (
	"path"
)

func init() {
	RegisterModule(&AvailableTasks, "CustomExporterServer", "/settings/CustomExporter", NewCustomExporterHandler)
}

type CustomExporterHandler struct {
	noCopy noCopy
	snc    *Agent
}

func NewCustomExporterHandler() Module {
	return &CustomExporterHandler{}
}

func (ch *CustomExporterHandler) Defaults() ConfigData {
	defaults := ConfigData{}

	return defaults
}

func (ch *CustomExporterHandler) Init(snc *Agent, _ *ConfigSection, conf *Config, _ *ModuleSet) error {
	ch.snc = snc

	err := ch.registerHandler(conf)
	if err != nil {
		return err
	}

	log.Tracef("custom exporter(s) initialized")

	return nil
}

func (ch *CustomExporterHandler) Start() error {
	return nil
}

func (ch *CustomExporterHandler) Stop() {
}

func (ch *CustomExporterHandler) registerHandler(conf *Config) error {
	// read all available exporters
	for sectionName := range conf.SectionsByPrefix("/settings/CustomExporter/") {
		name := path.Base(sectionName)
		if name == "default" {
			continue
		}
		expModule := func() Module {
			expModule := &HandlerNodeExporter{
				name: name,
			}

			return expModule
		}
		RegisterModule(&AvailableListeners, "CustomExporterServer", sectionName, expModule)
	}

	return nil
}
