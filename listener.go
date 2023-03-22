package snclient

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

// Listener is a generic tcp listener and handles all incoming connections.
type Listener struct {
	noCopy        noCopy
	snc           *SNClientInstance
	connType      string
	port          int64
	bindAddress   string
	tlsConfig     *tls.Config   // TODO: ...
	allowedHosts  []string      // TODO: ...
	socketTimeout time.Duration // TODO:...
	listen        net.Listener
	handler       RequestHandler
}

// NewListener creates a new Listener object.
func NewListener(snc *SNClientInstance, conf map[string]string, r RequestHandler) (*Listener, error) {
	l := Listener{
		snc:      snc,
		listen:   nil,
		handler:  r,
		connType: r.Type(),
	}

	if port, ok := conf["port"]; ok {
		num, err := strconv.ParseInt(port, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid port specification for %s: %w: %s", l.connType, err, err.Error())
		}

		l.port = num
	}

	l.bindAddress = conf["bind_to_address"]

	return &l, nil
}

// Start listening.
func (l *Listener) Start() error {
	log.Infof("starting %s listener on %s:%d", l.connType, l.bindAddress, l.port)

	listen, err := net.Listen("tcp", fmt.Sprintf("%s:%d", l.bindAddress, l.port))
	if err != nil {
		l.listen = nil

		return fmt.Errorf("listen failed: %w: %s", err, err.Error())
	}

	l.listen = listen

	go func() {
		for {
			con, err := listen.Accept()
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				log.Warnf("accept failed: %w: %s", err, err.Error())

				continue
			}

			if err != nil {
				log.Infof("stopping %s listener on %s:%d", l.connType, l.bindAddress, l.port)

				return
			}

			/* TODO: ...
			if timeout > 0 {
				con.SetReadDeadline(time.Now().Add(timeout))
			}
			*/

			// TODO: netfilter

			log.Debugf("incoming %s connection from %s", l.connType, con.RemoteAddr())
			l.handler.Handle(l.snc, con)
		}
	}()

	return nil
}

// Stop shuts down current listener.
func (l *Listener) Stop() {
	if l.listen != nil {
		l.listen.Close()
	}

	l.listen = nil
}
