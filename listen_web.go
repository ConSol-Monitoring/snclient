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
	noCopy noCopy
}

func NewHandlerWeb() *HandlerWeb {
	l := &HandlerWeb{}

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
		{URL: "*", Handler: l},
	}
}

func (l *HandlerWeb) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(res, "Hello, %q", html.EscapeString(req.URL.Path))
}
