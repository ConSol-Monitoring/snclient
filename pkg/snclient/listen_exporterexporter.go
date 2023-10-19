package snclient

import (
	"net/http"
	"strings"
)

func init() {
	RegisterModule(&AvailableListeners, "ExporterExporterServer", "/settings/ExporterExporter/server", NewHandlerExporterExporter)
}

type HandlerExporterExporter struct {
	noCopy    noCopy
	handler   http.Handler
	listener  *Listener
	urlPrefix string
}

func NewHandlerExporterExporter() Module {
	l := &HandlerExporterExporter{}
	l.handler = &HandlerWebExporterExporter{Handler: l}

	return l
}

func (l *HandlerExporterExporter) Type() string {
	return "exporterexporter"
}

func (l *HandlerExporterExporter) BindString() string {
	return l.listener.BindString()
}

func (l *HandlerExporterExporter) Start() error {
	return l.listener.Start()
}

func (l *HandlerExporterExporter) Stop() {
	l.listener.Stop()
}

func (l *HandlerExporterExporter) Defaults() ConfigData {
	defaults := ConfigData{
		"port":        "8443",
		"use ssl":     "1",
		"url prefix":  "/",
		"modules dir": "${shared-path}/exporter_modules",
	}
	defaults.Merge(DefaultListenHTTPConfig)

	return defaults
}

func (l *HandlerExporterExporter) Init(snc *Agent, conf *ConfigSection, _ *Config) error {
	listener, err := NewListener(snc, conf, l)
	if err != nil {
		return err
	}
	l.listener = listener
	urlPrefix, _ := conf.GetString("url prefix")
	l.urlPrefix = strings.TrimSuffix(urlPrefix, "/")

	return nil
}

func (l *HandlerExporterExporter) GetMappings(*Agent) []URLMapping {
	return []URLMapping{
		{URL: l.urlPrefix + "/", Handler: l.handler},
		{URL: l.urlPrefix + "/metrics", Handler: l.handler},
		{URL: l.urlPrefix + "/proxy", Handler: l.handler},
	}
}

type HandlerWebExporterExporter struct {
	noCopy  noCopy
	Handler *HandlerExporterExporter
}

func (l *HandlerWebExporterExporter) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/":
		res.WriteHeader(http.StatusOK)
	case "/metrics":
		res.WriteHeader(http.StatusNotFound)
		LogError2(res.Write([]byte("404 - /metrics is not implemented")))
	case "/proxy":
		res.WriteHeader(http.StatusOK)
		LogError2(res.Write([]byte("coming soon...")))
	default:
		res.WriteHeader(http.StatusNotFound)
		LogError2(res.Write([]byte("404 - nothing here")))
	}
}
