package snclient

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/consol-monitoring/snclient/pkg/utils"
	"github.com/go-chi/chi/v5"
)

// Listener is a generic tcp listener and handles all incoming connections.
type Listener struct {
	noCopy        noCopy
	snc           *Agent
	connType      string
	listen        net.Listener
	handler       []RequestHandler
	port          int64
	bindAddress   string
	tlsConfig     *tls.Config
	socketTimeout time.Duration
}

// NewListener creates a new Listener object.
func NewListener(snc *Agent, conf *ConfigSection, r RequestHandler) (*Listener, error) {
	listen := Listener{
		listen:   nil,
		snc:      snc,
		handler:  []RequestHandler{r},
		connType: r.Type(),
	}

	if err := listen.setListenConfig(conf); err != nil {
		return nil, err
	}

	return &listen, nil
}

// SharedWebListener returns a shared web Listener object.
func SharedWebListener(snc *Agent, conf *ConfigSection, webHandler RequestHandler, runSet *AgentRunSet) (*Listener, error) {
	listener, err := NewListener(snc, conf, webHandler)
	if err != nil {
		return nil, err
	}
	name := listener.BindString()
	existing := runSet.listeners.Get(name)
	if existing == nil {
		return listener, err
	}
	if reqHandler, ok := existing.(RequestHandler); ok {
		if handler := reqHandler.Listener(); handler != nil {
			handler.handler = append(handler.handler, webHandler)

			if (handler.tlsConfig == nil) != (listener.tlsConfig == nil) {
				return nil, fmt.Errorf("shared port configuration must also share tls/ssl configuration: %s: use ssl = %v vs. %s: use ssl = %v",
					handler.connType,
					handler.tlsConfig != nil,
					listener.connType,
					listener.tlsConfig != nil,
				)
			}

			return handler, nil
		}
	}

	return listener, err
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
	socketTimeout, ok, err := conf.GetDuration("timeout")
	switch {
	case err != nil:
		return fmt.Errorf("invalid timeout specification: %s", err.Error())
	case ok:
		l.socketTimeout = time.Duration(socketTimeout) * time.Second
	default:
		l.socketTimeout = DefaultSocketTimeout * time.Second
	}

	// parse / set ssl config
	useSsl, _, err := conf.GetBool("use ssl")
	switch {
	case err != nil:
		return fmt.Errorf("invalid use ssl specification: %s", err.Error())
	case useSsl:
		if err := l.setListenTLSConfig((conf)); err != nil {
			return err
		}
	}

	return nil
}

func (l *Listener) setListenTLSConfig(conf *ConfigSection) error {
	l.tlsConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// tls minimum version
	if tlsMin, ok := conf.GetString("tls min version"); ok {
		minV, err := utils.ParseTLSMinVersion(tlsMin)
		if err != nil {
			return fmt.Errorf("invalid tls min version: %s", err.Error())
		}
		l.tlsConfig.MinVersion = minV
	}

	/* remove insecure ciphers, but only tls == 1.2
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

	certKey := certPath
	if !strings.HasSuffix(certPath, ".pem") {
		certKey, ok = conf.GetString("certificate key")
		switch {
		case !ok:
			return fmt.Errorf("invalid ssl configuration, ssl enabled but no certificate key set")
		case ok:
			_, err := os.ReadFile(certKey)
			if err != nil {
				return fmt.Errorf("cannot read certificate key: %s", err.Error())
			}
		}
	}
	cer, err := tls.LoadX509KeyPair(certPath, certKey)
	if err != nil {
		return fmt.Errorf("tls.LoadX509KeyPair: %s / %s: %s", certPath, certKey, err.Error())
	}
	l.tlsConfig.Certificates = []tls.Certificate{cer}

	caCerts := []string{}
	// ca option for backward compatibility with NSclient
	caPEMs, ok := conf.GetString("ca")
	if ok {
		caCerts = append(caCerts, strings.Split(caPEMs, ",")...)
	}

	clientPEMs, ok := conf.GetString("client certificates")
	if ok {
		caCerts = append(caCerts, strings.Split(clientPEMs, ",")...)
	}

	// require client certificate verification
	// only used if CA certificates specified by "ca" or "client certificates" options
	if len(caCerts) > 0 {
		caCertPool := x509.NewCertPool()
		for _, file := range caCerts {
			file = strings.TrimSpace(file)
			caCert, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("os.ReadFile: %w", err)
			}
			caCertPool.AppendCertsFromPEM(caCert)
		}

		l.tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		l.tlsConfig.ClientCAs = caCertPool
	}

	return nil
}

// Stop shuts down current listener.
func (l *Listener) Stop() {
	if l.listen != nil {
		l.listen.Close()
		l.listen = nil
	}
}

// Start listening.
func (l *Listener) Start() error {
	if l.listen != nil {
		log.Tracef("listener %s on %s already running", l.connType, l.BindString())

		return nil
	}

	for _, hdl := range l.handler {
		log.Infof("starting %s listener on %s", hdl.Type(), l.BindString())
		sslOptions := ""
		if l.tlsConfig != nil && l.tlsConfig.ClientCAs != nil {
			sslOptions = " (client certificate required)"
		}

		log.Debugf("ssl: %v%s", l.tlsConfig != nil, sslOptions)
		allowed := hdl.GetAllowedHosts()
		allowed.Debug()
	}

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

	switch handler := l.handler[0].(type) {
	case RequestHandlerHTTP:
		go func() {
			defer l.snc.logPanicExit()

			l.startListenerHTTP(l.handler)
		}()
	case RequestHandlerTCP:
		go func() {
			defer l.snc.logPanicExit()

			l.startListenerTCP(handler)
		}()
	default:
		return fmt.Errorf("unsupported type: %T (does not implement any known request handler)", l.handler[0])
	}

	return nil
}

func (l *Listener) startListenerTCP(handler RequestHandlerTCP) {
	for {
		if l.listen == nil {
			log.Infof("stopping %s listener on %s", l.connType, l.BindString())

			return
		}
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

		go func(con net.Conn) {
			// log panics during request, but continue listening
			defer l.snc.logPanicRecover()

			l.handleTCPCon(con, handler)
		}(con)
	}
}

func (l *Listener) handleTCPCon(con net.Conn, handler RequestHandlerTCP) {
	startTime := time.Now()

	log.Tracef("incoming %s connection from %s", l.connType, con.RemoteAddr().String())

	allowed := handler.GetAllowedHosts()
	if !allowed.Check(con.RemoteAddr().String()) {
		log.Warnf("ip %s is not in the allowed hosts", con.RemoteAddr().String())
		con.Close()

		return
	}

	if err := con.SetReadDeadline(time.Now().Add(l.socketTimeout)); err != nil {
		log.Warnf("setting timeout on %s client connection failed: %s", l.connType, err.Error())
	}

	handler.ServeTCP(l.snc, con)

	duration := time.Since(startTime)
	name := handler.Type()
	promTCPRequestsTotal.WithLabelValues(name).Add(1)
	promTCPDuration.WithLabelValues(name).Observe(duration.Seconds())
	log.Debugf("%s connection from %s finished in %9s", l.connType, con.RemoteAddr().String(), duration)
}

func (l *Listener) startListenerHTTP(handler []RequestHandler) {
	mux := chi.NewRouter()

	// Add generic logger and connection checker
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer l.snc.logPanicExit()

			l.LogWrapHTTPHandler(next, w, r)
		})
	})

	mappingsInUse := map[string]string{}
	for _, hdl := range handler {
		if webHandler, ok := hdl.(RequestHandlerHTTP); ok {
			mappings := webHandler.GetMappings(l.snc)
			for i := range mappings {
				mapping := mappings[i]
				log.Tracef("mapping port %-6s handler: %-16s url: %s", webHandler.BindString(), webHandler.Type(), mapping.URL)
				if prev, ok := mappingsInUse[mapping.URL]; ok {
					log.Warnf("url %s is mapped multiple times (previously assigned to %s), use url prefix to avoid this.", mapping.URL, prev)
				}
				mappingsInUse[mapping.URL] = webHandler.Type()
				mux.HandleFunc(mapping.URL, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer l.snc.logPanicExit()

					l.WrappedCheckHTTPHandler(webHandler, &mapping, w, r)
				}))
			}
		}
	}

	log.Tracef("http default read timeout: %s", l.socketTimeout.String())
	server := &http.Server{
		ReadTimeout:       0,
		ReadHeaderTimeout: l.socketTimeout,
		WriteTimeout:      0,
		IdleTimeout:       l.socketTimeout,
		Handler:           mux,
		ErrorLog:          NewStandardLog("WARN"),
	}

	if l.listen == nil {
		log.Error("listener for %s is nil", l.bindAddress)

		return
	}

	if err := server.Serve(l.listen); err != nil {
		log.Tracef("http server finished: %s", err.Error())
	}
}

func (l *Listener) BindString() string {
	return (fmt.Sprintf("%s:%d", l.bindAddress, l.port))
}

// log wrapper for all web requests
func (l *Listener) LogWrapHTTPHandler(next http.Handler, res http.ResponseWriter, req *http.Request) {
	startTime := time.Now()

	log.Tracef("incoming http(s) connection from %s", req.RemoteAddr)

	// log panics during request, but continue listening
	defer l.snc.logPanicRecover()

	logHTTPRequest(req)

	resCapture := &ResponseWriterCapture{
		w:           res,
		captureBody: log.IsV(LogVerbosityTrace2),
	}
	res = resCapture
	next.ServeHTTP(res, req)

	if log.IsV(LogVerbosityTrace2) {
		if capture, ok := res.(*ResponseWriterCapture); ok {
			log.Tracef("http response:\n%s", capture.String(req, true))
		}
	}

	duration := time.Since(startTime)
	promHTTPRequestsTotal.WithLabelValues(fmt.Sprintf("%d", resCapture.statusCode), req.URL.Path).Add(1)
	promHTTPDuration.WithLabelValues(fmt.Sprintf("%d", resCapture.statusCode), req.URL.Path).Observe(duration.Seconds())

	log.Debugf("http(s) request finished from: %-20s | duration: %12s | code: %3d | %s %s",
		req.RemoteAddr,
		duration,
		resCapture.statusCode,
		req.Method,
		req.URL.Path,
	)
}

// wrapper for all known web requests to verify passwords and allowed hosts
func (l *Listener) WrappedCheckHTTPHandler(webHandler RequestHandlerHTTP, mapping *URLMapping, res http.ResponseWriter, req *http.Request) {
	allowed := webHandler.GetAllowedHosts()
	if !allowed.Check(req.RemoteAddr) {
		log.Warnf("ip %s is not in the allowed hosts", req.RemoteAddr)
		http.Error(res, http.StatusText(http.StatusForbidden), http.StatusForbidden)

		return
	}

	if !webHandler.CheckPassword(req, *mapping) {
		http.Error(res, http.StatusText(http.StatusForbidden), http.StatusForbidden)

		return
	}

	mapping.Handler.ServeHTTP(res, req)
}
