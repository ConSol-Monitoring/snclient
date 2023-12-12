package snclient

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	case "/api/v1/admin/certs/replace":
		l.serveCertsReplace(res, req)
	default:
		res.WriteHeader(http.StatusNotFound)
		LogError2(res.Write([]byte("404 - nothing here\n")))
	}
}

func (l *HandlerWebAdmin) serveReload(res http.ResponseWriter, req *http.Request) {
	if !l.requirePostMethod(res, req) {
		return
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	LogError(json.NewEncoder(res).Encode(map[string]interface{}{
		"success": true,
	}))
	l.Handler.snc.osSignalChannel <- syscall.SIGHUP
}

func (l *HandlerWebAdmin) serveCertsReplace(res http.ResponseWriter, req *http.Request) {
	if !l.requirePostMethod(res, req) {
		return
	}

	// extract json payload
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	type postData struct {
		CertData string
		KeyData  string
		Reload   bool
	}
	data := postData{}
	err := decoder.Decode(&data)
	if err != nil {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusBadRequest)
		LogError(json.NewEncoder(res).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}))

		return
	}

	certBytes := []byte{}
	keyBytes := []byte{}
	if data.CertData != "" {
		certBytes, err = base64.StdEncoding.DecodeString(data.CertData)
		if err != nil {
			l.sendError(res, fmt.Errorf("failed to base64 decode certdata: %s", err.Error()))

			return
		}
	}

	if data.KeyData != "" {
		keyBytes, err = base64.StdEncoding.DecodeString(data.KeyData)
		if err != nil {
			l.sendError(res, fmt.Errorf("failed to base64 decode keydata: %s", err.Error()))

			return
		}
	}

	defSection := l.Handler.snc.Config.Section("/settings/default")
	certFile, _ := defSection.GetString("certificate")
	keyFile, _ := defSection.GetString("certificate key")

	if data.CertData != "" {
		if err := os.WriteFile(certFile, certBytes, 0o600); err != nil {
			l.sendError(res, fmt.Errorf("failed to write certificate %s: %s", certFile, err.Error()))

			return
		}
	}

	if data.KeyData != "" {
		if err := os.WriteFile(keyFile, keyBytes, 0o600); err != nil {
			l.sendError(res, fmt.Errorf("failed to write certificate key file %s: %s", keyFile, err.Error()))

			return
		}
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	LogError(json.NewEncoder(res).Encode(map[string]interface{}{
		"success": true,
	}))

	if data.Reload {
		l.Handler.snc.osSignalChannel <- syscall.SIGHUP
	}
}

// check if request used method POST
func (l *HandlerWebAdmin) requirePostMethod(res http.ResponseWriter, req *http.Request) bool {
	if req.Method == http.MethodPost {
		return true
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusBadRequest)
	LogError(json.NewEncoder(res).Encode(map[string]interface{}{
		"success": false,
		"error":   "POST method required",
	}))

	return false
}

// return error as json result
func (l *HandlerWebAdmin) sendError(res http.ResponseWriter, err error) {
	log.Debugf("admin request failed: %s", err.Error())
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusInternalServerError)
	LogError(json.NewEncoder(res).Encode(map[string]interface{}{
		"success": false,
		"error":   err.Error(),
	}))
}
