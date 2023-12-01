package snclient

import (
	"encoding/json"
	"net/http"
	"strings"
	"syscall"
)

func init() {
	RegisterModule(&AvailableListeners, "WEBAdminServer", "/settings/WEBAdmin/server", NewHandlerAdmin)
}

type HandlerAdmin struct {
	noCopy       noCopy
	handler      http.Handler
	password     string
	snc          *Agent
	listener     *Listener
	allowedHosts *AllowedHostConfig
}

// ensure we fully implement the RequestHandlerHTTP type
var _ RequestHandlerHTTP = &HandlerAdmin{}

func NewHandlerAdmin() Module {
	l := &HandlerAdmin{}
	l.handler = &HandlerWebAdmin{Handler: l}

	return l
}

func (l *HandlerAdmin) Type() string {
	return "admin"
}

func (l *HandlerAdmin) BindString() string {
	return l.listener.BindString()
}

func (l *HandlerAdmin) Listener() *Listener {
	return l.listener
}

func (l *HandlerAdmin) Start() error {
	return l.listener.Start()
}

func (l *HandlerAdmin) Stop() {
	if l.listener != nil {
		l.listener.Stop()
	}
}

func (l *HandlerAdmin) Defaults() ConfigData {
	defaults := ConfigData{
		"port":            "8443",
		"use ssl":         "1",
		"allow arguments": "true",
	}
	defaults.Merge(DefaultListenHTTPConfig)

	return defaults
}

func (l *HandlerAdmin) Init(snc *Agent, conf *ConfigSection, _ *Config, set *ModuleSet) error {
	l.snc = snc
	l.password = DefaultPassword
	if password, ok := conf.GetString("password"); ok {
		l.password = password
	}

	listener, err := SharedWebListener(snc, conf, l, set)
	if err != nil {
		return err
	}
	l.listener = listener

	allowedHosts, err := NewAllowedHostConfig(conf)
	if err != nil {
		return err
	}
	l.allowedHosts = allowedHosts

	return nil
}

func (l *HandlerAdmin) GetAllowedHosts() *AllowedHostConfig {
	return l.allowedHosts
}

func (l *HandlerAdmin) CheckPassword(req *http.Request, _ URLMapping) bool {
	return verifyRequestPassword(l.snc, req, l.password)
}

func (l *HandlerAdmin) GetMappings(*Agent) []URLMapping {
	return []URLMapping{
		{URL: "/api/v1/admin/*", Handler: l.handler},
	}
}

type HandlerWebAdmin struct {
	noCopy  noCopy
	Handler *HandlerAdmin
}

func (l *HandlerWebAdmin) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	path := strings.TrimSuffix(req.URL.Path, "/")
	switch path {
	case "/api/v1/admin/reload":
		l.serveReload(res, req)
	default:
		res.WriteHeader(http.StatusNotFound)
		LogError2(res.Write([]byte("404 - nothing here\n")))
	}
}

func (l *HandlerWebAdmin) serveReload(res http.ResponseWriter, _ *http.Request) {
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	LogError(json.NewEncoder(res).Encode(map[string]interface{}{
		"success": true,
	}))
	l.Handler.snc.osSignalChannel <- syscall.SIGHUP
}
