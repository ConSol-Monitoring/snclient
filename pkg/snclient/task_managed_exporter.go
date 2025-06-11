package snclient

import (
	"path"
)

var defaultManagedExporterConfig = ConfigData{
	"agent path":       "",
	"agent args":       "",
	"agent address":    "127.0.0.1:9990",
	"agent max memory": "256M",
	"agent user":       "",
	"port":             "${/settings/WEB/server/port}",
	"use ssl":          "${/settings/WEB/server/use ssl}",
	"url prefix":       "",
	"kill orphaned":    "enabled",
}

func init() {
	RegisterModule(&AvailableTasks, "ManagedExporterServer", "/settings/ManagedExporter", NewManagedExporterHandler, nil)
}

type ManagedExporterHandler struct {
	noCopy noCopy
	snc    *Agent
}

func NewManagedExporterHandler() Module {
	return &ManagedExporterHandler{}
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

		RegisterModule(
			&AvailableListeners,
			"ManagedExporterServer",
			sectionName,
			expModule,
			ConfigInit{
				defaultManagedExporterConfig,
				"/settings/default",
				DefaultListenHTTPConfig,
			})
		// since config initialization was done already, apply defaults for this section
		conf.ApplyMergeDefaultsKey(sectionName, moduleConfigDefaults)
		nr++
	}

	return nr
}
