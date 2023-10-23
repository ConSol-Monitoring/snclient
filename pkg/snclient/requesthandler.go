package snclient

import (
	"net"
	"net/http"
)

// RequestHandler handles a client connections.
type RequestHandler interface {
	Module
	Type() string
	BindString() string
	Listener() *Listener
}

// RequestHandlerTCP handles a single client connection.
type RequestHandlerTCP interface {
	RequestHandler
	ServeTCP(snc *Agent, conn net.Conn)
}

type URLMapping struct {
	URL     string
	Handler http.Handler
}

// RequestHandlerHTTP handles a single client connection using http(s).
type RequestHandlerHTTP interface {
	RequestHandler
	GetMappings(snc *Agent) []URLMapping
}
