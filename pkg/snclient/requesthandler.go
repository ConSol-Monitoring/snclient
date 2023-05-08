package snclient

import (
	"net"
	"net/http"
)

// RequestHandler handles a client connections.
type RequestHandler interface {
	Module
	Type() string
}

// RequestHandlerTCP handles a single client connection.
type RequestHandlerTCP interface {
	RequestHandler
	ServeTCP(*Agent, net.Conn)
}

type URLMapping struct {
	URL     string
	Handler http.Handler
}

// RequestHandlerHTTP handles a single client connection using http(s).
type RequestHandlerHTTP interface {
	RequestHandler
	GetMappings(*Agent) []URLMapping
}
