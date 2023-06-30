package snclient

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"utils"

	"github.com/go-chi/chi/v5"
)

var DefaultListenTCPConfig = ConfigData{
	"allowed hosts":       "127.0.0.1, [::1]",
	"bind to":             "",
	"cache allowed hosts": "1",
	"certificate":         "${certificate-path}/server.crt",
	"certificate key":     "${certificate-path}/server.key",
	"timeout":             "30",
	"use ssl":             "0",
}

var DefaultListenHTTPConfig = ConfigData{}

func init() {
	DefaultListenHTTPConfig.Merge(DefaultListenTCPConfig)
}

// Listener is a generic tcp listener and handles all incoming connections.
type Listener struct {
	noCopy            noCopy
	snc               *Agent
	connType          string
	listen            net.Listener
	handler           RequestHandler
	port              int64
	bindAddress       string
	cacheAllowedHosts bool
	tlsConfig         *tls.Config
	allowedHosts      []AllowedHost
	socketTimeout     time.Duration
}

// NewListener creates a new Listener object.
func NewListener(snc *Agent, conf *ConfigSection, r RequestHandler) (*Listener, error) {
	listen := Listener{
		listen:   nil,
		snc:      snc,
		handler:  r,
		connType: r.Type(),
	}

	if err := listen.setListenConfig(conf); err != nil {
		return nil, err
	}

	return &listen, nil
}

func (l *Listener) setListenConfig(conf *ConfigSection) error {
	// parse/set port.
	port, ok := conf.GetString("port")
	if ok {
		if strings.HasSuffix(port, "s") {
			port = strings.TrimSuffix(port, "s")
			conf.Set("use ssl", "1")
		}

		num, err := strconv.ParseInt(port, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid port specification: %s", err.Error())
		}

		l.port = num
	}

	// set bind address (can be empty)
	bindAddress, _ := conf.GetString("bind to")
	l.bindAddress = bindAddress

	// parse / set socket timeout.
	socketTimeout, ok, err := conf.GetInt("timeout")
	switch {
	case err != nil:
		return fmt.Errorf("invalid timeout specification: %s", err.Error())
	case ok:
		l.socketTimeout = time.Duration(socketTimeout) * time.Second
	default:
		l.socketTimeout = DefaultSocketTimeout * time.Second
	}

	// parse / set allowed hosts
	allowed, _ := conf.GetString("allowed hosts")
	if allowed != "" {
		for _, allow := range strings.Split(allowed, ",") {
			allow = strings.TrimSpace(allow)
			if allow == "" {
				continue
			}
			l.allowedHosts = append(l.allowedHosts, NewAllowedHost(allow))
		}
	}

	// parse / set cache allowed hosts
	cacheAllowedHosts, ok, err := conf.GetBool("cache allowed hosts")
	switch {
	case err != nil:
		return fmt.Errorf("invalid cache allowed hosts specification: %s", err.Error())
	case ok:
		l.cacheAllowedHosts = cacheAllowedHosts
	default:
		l.cacheAllowedHosts = true
	}

	// parse / set ssl config
	useSsl, _, err := conf.GetBool("use ssl")
	switch {
	case err != nil:
		return fmt.Errorf("invalid use ssl specification: %s", err.Error())
	case useSsl:
		l.tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		// tls minimum version
		if tlsMin, ok := conf.GetString("tls min version"); ok {
			min, err := utils.ParseTLSMinVersion(tlsMin)
			if err != nil {
				return fmt.Errorf("invalid tls min version: %s", err.Error())
			}
			l.tlsConfig.MinVersion = min
		}

		/* remove insecure cipers, but only tls == 1.2
		 * with tls 1.3 go decides which ciphers will be used
		 * with tls < 1.2 we allow all ciphers, it unsecure anyway and it seems like an old client needs to connect (default is 1.2)
		 */
		if l.tlsConfig.MinVersion == tls.VersionTLS12 {
			l.tlsConfig.CipherSuites = utils.GetSecureCiphers()
		}

		// certificate
		certPath, ok := conf.GetString("certificate")
		switch {
		case !ok:
			return fmt.Errorf("invalid ssl configuration, ssl enabled but no certificate set")
		case ok:
			_, err := os.ReadFile(certPath)
			if err != nil {
				return fmt.Errorf("cannot read certificate: %s", err.Error())
			}
		}

		certKey, ok := conf.GetString("certificate key")
		switch {
		case !ok:
			return fmt.Errorf("invalid ssl configuration, ssl enabled but no certificate key set")
		case ok:
			_, err := os.ReadFile(certKey)
			if err != nil {
				return fmt.Errorf("cannot read certificate key: %s", err.Error())
			}
		}
		cer, err := tls.LoadX509KeyPair(certPath, certKey)
		if err != nil {
			return fmt.Errorf("tls.LoadX509KeyPair: %s / %s: %s", certPath, certKey, err.Error())
		}
		l.tlsConfig.Certificates = []tls.Certificate{cer}
	}

	return nil
}

// Stop shuts down current listener.
func (l *Listener) Stop() {
	if l.listen != nil {
		l.listen.Close()
	}
}

// Start listening.
func (l *Listener) Start() error {
	log.Infof("starting %s listener on %s", l.connType, l.BindString())
	log.Debugf("ssl: %v", l.tlsConfig != nil)

	if len(l.allowedHosts) == 0 {
		log.Debugf("allowed hosts: all")
	} else {
		log.Debugf("allowed hosts:")
		for _, allow := range l.allowedHosts {
			log.Debugf("    - %s", allow.String())
		}
	}

	l.listen = nil

	if l.tlsConfig != nil {
		listen, err := tls.Listen("tcp", l.BindString(), l.tlsConfig)
		if err != nil {
			return fmt.Errorf("tls listen failed: %s", err.Error())
		}
		l.listen = listen
	} else {
		listen, err := net.Listen("tcp", l.BindString())
		if err != nil {
			return fmt.Errorf("listen failed: %s", err.Error())
		}
		l.listen = listen
	}

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
				log.Warnf("accept failed: %s", err.Error())

				continue
			}

			log.Infof("stopping %s listener on %s", l.connType, l.BindString())

			return
		}

		l.handleTCPCon(con, handler)
	}
}

func (l *Listener) handleTCPCon(con net.Conn, handler RequestHandlerTCP) {
	startTime := time.Now()

	log.Tracef("incoming %s connection from %s", l.connType, con.RemoteAddr().String())

	if !l.CheckConnection(con) {
		con.Close()

		return
	}

	if err := con.SetReadDeadline(time.Now().Add(l.socketTimeout)); err != nil {
		log.Warnf("setting timeout on %s listener failed: %s", err.Error())
	}

	handler.ServeTCP(l.snc, con)

	log.Debugf("%s connection from %s finished in %9s", l.connType, con.RemoteAddr().String(), time.Since(startTime))
}

func (l *Listener) startListenerHTTP(handler RequestHandlerHTTP) {
	mux := chi.NewRouter()

	// Add generic logger.
	mappings := handler.GetMappings(l.snc)
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			l.WrappedHTTPHandler(next, w, r)
		})
	})
	for _, mapping := range mappings {
		mux.Handle(mapping.URL, mapping.Handler)
	}

	server := &http.Server{
		ReadTimeout:       DefaultSocketTimeout * time.Second,
		ReadHeaderTimeout: DefaultSocketTimeout * time.Second,
		WriteTimeout:      DefaultSocketTimeout * time.Second,
		IdleTimeout:       DefaultSocketTimeout * time.Second,
		Handler:           mux,
		ErrorLog:          NewStandardLog("WARN"),
		ConnState: func(con net.Conn, state http.ConnState) {
			if state != http.StateNew {
				return
			}

			log.Tracef("incoming %s connection from %s", l.connType, con.RemoteAddr().String())

			if !l.CheckConnection(con) {
				con.Close()

				return
			}
		},
	}

	if err := server.Serve(l.listen); err != nil {
		log.Tracef("http server finished: %s", err.Error())
	}
}

func (l *Listener) CheckConnection(con net.Conn) bool {
	if !l.CheckAllowedHosts(con.RemoteAddr().String()) {
		log.Warnf("%s connection from %s not allowed", l.handler.Type(), con.RemoteAddr().String())

		return false
	}

	return true
}

func (l *Listener) CheckAllowedHosts(remoteAddr string) bool {
	if len(l.allowedHosts) == 0 {
		return true
	}

	idx := strings.LastIndex(remoteAddr, ":")
	if idx != -1 {
		remoteAddr = remoteAddr[:idx]
	}

	if strings.HasPrefix(remoteAddr, "[") && strings.HasSuffix(remoteAddr, "]") {
		remoteAddr = strings.TrimPrefix(remoteAddr, "[")
		remoteAddr = strings.TrimSuffix(remoteAddr, "]")
	}

	addr, err := netip.ParseAddr(remoteAddr)
	if err != nil {
		log.Warnf("cannot parse remote address: %s: %s", remoteAddr, err.Error())

		return false
	}

	for _, allow := range l.allowedHosts {
		if allow.Contains(addr, l.cacheAllowedHosts) {
			return true
		}
	}

	return false
}

func (l *Listener) BindString() string {
	return (fmt.Sprintf("%s:%d", l.bindAddress, l.port))
}

func (l *Listener) WrappedHTTPHandler(next http.Handler, res http.ResponseWriter, req *http.Request) {
	startTime := time.Now()

	if log.IsV(LogVerbosityTrace) {
		reqStr, err := httputil.DumpRequest(req, true)
		if err != nil {
			log.Tracef("%s", err.Error())
		} else {
			log.Tracef("http request:\n%s", string(reqStr))
		}
	}

	if log.IsV(LogVerbosityTrace) {
		resCapture := &ResponseWriterCapture{
			w: res,
		}
		res = resCapture
	}
	next.ServeHTTP(res, req)

	if capture, ok := res.(*ResponseWriterCapture); ok {
		log.Tracef("http response:\n%s", capture.String(req, true))
	}

	log.Debugf("%s connection from %s finished in %9s", l.connType, req.RemoteAddr, time.Since(startTime))
}
