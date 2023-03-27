package snclient

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

// Listener is a generic tcp listener and handles all incoming connections.
type Listener struct {
	noCopy        noCopy
	snc           *Agent
	connType      string
	port          int64
	bindAddress   string
	tlsConfig     *tls.Config
	allowedHosts  []*netip.Prefix
	socketTimeout time.Duration
	listen        net.Listener
	handler       RequestHandler
}

// NewListener creates a new Listener object.
func NewListener(snc *Agent, conf map[string]string, r RequestHandler) (*Listener, error) {
	listen := Listener{
		snc:      snc,
		listen:   nil,
		handler:  r,
		connType: r.Type(),
	}

	// parse/set port.
	if port, ok := conf["port"]; ok {
		num, err := strconv.ParseInt(port, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid port specification for %s: %w: %s", listen.connType, err, err.Error())
		}

		listen.port = num
	}

	// set bind address (can be empty)
	listen.bindAddress = conf["bind_to_address"]

	// parse / set socket timeout.
	listen.socketTimeout = DefaulSocketTimeout * time.Second

	if socketTimeout, ok := conf["socket_timeout"]; ok {
		num, err := strconv.ParseInt(socketTimeout, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid socket_timeout specification for %s: %w: %s", listen.connType, err, err.Error())
		}

		listen.socketTimeout = time.Duration(num) * time.Second
	}

	// parse / set allowed hosts
	if allowed, ok := conf["allowed_hosts"]; ok {
		for _, allow := range strings.Split(allowed, ",") {
			allow = strings.TrimSpace(allow)
			if allow == "" {
				continue
			}

			netrange, err := netip.ParsePrefix(allow)
			if err != nil {
				return nil, fmt.Errorf("invalid allowed_hosts specification for %s: %w: %s", listen.connType, err, err.Error())
			}

			listen.allowedHosts = append(listen.allowedHosts, &netrange)
		}
	}

	return &listen, nil
}

// Start listening.
func (l *Listener) Start() error {
	log.Infof("starting %s listener on %s:%d", l.connType, l.bindAddress, l.port)
	log.Debugf("ssl: %v", l.tlsConfig != nil)

	if len(l.allowedHosts) == 0 {
		log.Debugf("allowed_hosts: all")
	} else {
		log.Debugf("allowed_hosts:")
		for _, allow := range l.allowedHosts {
			log.Debugf("    - %s", allow.String())
		}
	}

	l.listen = nil

	listen, err := net.Listen("tcp", fmt.Sprintf("%s:%d", l.bindAddress, l.port))
	if err != nil {
		return fmt.Errorf("listen failed: %w: %s", err, err.Error())
	}

	l.listen = listen

	switch handler := l.handler.(type) {
	case RequestHandlerHTTP:
		go func() {
			defer l.snc.logPanicExit()

			l.startListenerHTTP(handler)
		}()
	case RequestHandlerTCP:
		go func() {
			defer l.snc.logPanicExit()

			l.startListenerTCP(handler)
		}()
	default:
		return fmt.Errorf("unsupported type: %T (does not implement any known request handler)", l.handler)
	}

	return nil
}

func (l *Listener) startListenerTCP(handler RequestHandlerTCP) {
	for {
		con, err := l.listen.Accept()

		var opErr *net.OpError

		if err != nil {
			if errors.As(err, &opErr) && opErr.Timeout() {
				log.Warnf("accept failed: %w: %s", err, err.Error())

				continue
			}

			log.Infof("stopping %s listener on %s:%d", l.connType, l.bindAddress, l.port)

			return
		}

		err = con.SetReadDeadline(time.Now().Add(l.socketTimeout))
		if err != nil {
			log.Warnf("setting timeout on %s listener failed: %s", err.Error())
		}

		l.ServeOne(con.RemoteAddr().String(), func() {
			handler.ServeTCP(l.snc, con)
		})
	}
}

func (l *Listener) startListenerHTTP(handler RequestHandlerHTTP) {
	mux := http.NewServeMux()

	// Wrap handler to apply netfilter and logger.
	mappings := handler.GetMappings(l.snc)
	for _, mapping := range mappings {
		mux.Handle(mapping.URL, l.WrapHTTPHandler(mapping.Handler))
	}

	server := &http.Server{
		ReadTimeout:       DefaulSocketTimeout * time.Second,
		ReadHeaderTimeout: DefaulSocketTimeout * time.Second,
		WriteTimeout:      DefaulSocketTimeout * time.Second,
		IdleTimeout:       DefaulSocketTimeout * time.Second,
		Handler:           mux,
	}

	if err := server.Serve(l.listen); err != nil {
		log.Tracef("http server finished: %s", err.Error())
	}
}

func (l *Listener) ServeOne(remoteAddr string, serveCB func()) {
	log.Debugf("incoming %s connection from %s", l.connType, remoteAddr)

	if !l.CheckAllowedHosts(remoteAddr) {
		log.Warnf("%s connection from %s prohibited by allowed_hosts", l.connType, remoteAddr)

		return
	}

	serveCB()
	log.Debugf("%s connection from %s finished", l.connType, remoteAddr)
}

func (l *Listener) CheckAllowedHosts(remoteAddr string) bool {
	if len(l.allowedHosts) == 0 {
		return true
	}

	idx := strings.LastIndex(remoteAddr, ":")
	if idx != -1 {
		remoteAddr = remoteAddr[:idx]
	}

	addr, err := netip.ParseAddr(remoteAddr)
	if err != nil {
		log.Warnf("cannot parse remote address: %s: %s", remoteAddr, err.Error())

		return false
	}

	for _, netrange := range l.allowedHosts {
		if netrange.Contains(addr) {
			return true
		}
	}

	return false
}

type WrappedHTTPHandler struct {
	listener *Listener
	handle   http.Handler
}

func (w *WrappedHTTPHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	w.listener.ServeOne(req.RemoteAddr, func() {
		w.handle.ServeHTTP(res, req)
	})
}

func (l *Listener) WrapHTTPHandler(handle http.Handler) http.Handler {
	return (&WrappedHTTPHandler{
		listener: l,
		handle:   handle,
	})
}

// Stop shuts down current listener.
func (l *Listener) Stop() {
	if l.listen != nil {
		l.listen.Close()
	}

	l.listen = nil
}
