package snclient

import (
	"net"
)

// RequestHandler handles a single client connection.
type RequestHandler interface {
	Type() string
	Defaults() map[string]string
	Init(*SNClientInstance) error
	Handle(*SNClientInstance, net.Conn)
}
