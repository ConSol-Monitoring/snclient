package snclient

import (
	"net"
	"net/http"
)

// RequestHandler handles a client connections.
type RequestHandler interface {
	Type() string
	Defaults() map[string]string
	Init(*SNClientInstance) error
}

// RequestHandlerTCP handles a single client connection.
type RequestHandlerTCP interface {
	RequestHandler
	ServeOne(*SNClientInstance, net.Conn)
}

type UrlMapping struct {
	Url     string
	Handler *http.Handler
}

// RequestHandlerHTTP handles a single client connection using http(s).
type RequestHandlerHTTP interface {
	RequestHandler
	GetMappings(*SNClientInstance) []UrlMapping
}
