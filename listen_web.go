package snclient

import (
	"fmt"
	"html"
	"net/http"
)

func init() {
	AvailableListeners = append(AvailableListeners, ListenHandler{"WEBServer", "/settings/WEB/server", NewHandlerWeb()})
}

type HandlerWeb struct {
	noCopy        noCopy
	handlerLegacy http.Handler
	handlerV1     http.Handler
}

func NewHandlerWeb() *HandlerWeb {
	l := &HandlerWeb{}
	l.handlerLegacy = &HandlerWebLegacy{Handler: l}
	l.handlerV1 = &HandlerWebV1{Handler: l}

	return l
}

func (l *HandlerWeb) Type() string {
	return "web"
}

func (l *HandlerWeb) Defaults() ConfigSection {
	defaults := ConfigSection{
		"port":    "8443",
		"use ssl": "1",
	}
	defaults.Merge(DefaultListenHTTPConfig)

	return defaults
}

func (l *HandlerWeb) Init(_ *Agent) error {
	return nil
}

func (l *HandlerWeb) GetMappings(*Agent) []URLMapping {
	return []URLMapping{
		{URL: "/query/", Handler: l.handlerLegacy},
		{URL: "/api/v1/queries/", Handler: l.handlerV1},
	}
}

type HandlerWebLegacy struct {
	noCopy  noCopy
	Handler *HandlerWeb
}

func (l *HandlerWebLegacy) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(res, "Hello from Legacy, %q\n", html.EscapeString(req.URL.Path))
}

type HandlerWebV1 struct {
	noCopy  noCopy
	Handler *HandlerWeb
}

func (l *HandlerWebV1) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(res, "Hello from V1, %q\n", html.EscapeString(req.URL.Path))
}
