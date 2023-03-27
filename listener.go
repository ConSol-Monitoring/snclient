package snclient

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

type ListenHandler struct {
	ModuleKey string
	ConfigKey string
	Init      RequestHandler
}

var DefaultListenTCPConfig = ConfigSection{
	"allowed ciphers":     "ALL:!ADH:!LOW:!EXP:!MD5:@STRENGTH",
	"allowed hosts":       "127.0.0.1, [::1]",
	"bind to":             "",
	"cache allowed hosts": "1",
	"certificate":         "${certificate-path}/certificate.pem",
	"timeout":             "30",
	"use ssl":             "0",
}

var DefaultListenHTTPConfig = ConfigSection{
	"password": "",
}

func init() {
	DefaultListenHTTPConfig.Merge(DefaultListenTCPConfig)
}

var AvailableListeners []ListenHandler

// Listener is a generic tcp listener and handles all incoming connections.
type Listener struct {
	noCopy            noCopy
	snc               *Agent
	connType          string
	port              int64
	bindAddress       string
	cacheAllowedHosts bool
	tlsConfig         *tls.Config
	allowedHosts      []AllowedHost
	socketTimeout     time.Duration
	listen            net.Listener
	handler           RequestHandler
}

// NewListener creates a new Listener object.
func NewListener(snc *Agent, conf ConfigSection, r RequestHandler) (*Listener, error) {
	listen := Listener{
		snc:      snc,
		listen:   nil,
		handler:  r,
		connType: r.Type(),
	}

	if err := listen.setListenConfig(conf); err != nil {
		return nil, err
	}

	return &listen, nil
}

func (l *Listener) setListenConfig(conf ConfigSection) error {
	// parse/set port.
	if port, ok := conf["port"]; ok {
		if strings.HasSuffix(port, "s") {
			port = strings.TrimSuffix(port, "s")
			conf["ssl"] = "1"
		}

		num, err := strconv.ParseInt(port, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid port specification: %s", err.Error())
		}

		l.port = num
	}

	// set bind address (can be empty)
	l.bindAddress = conf["bind to"]

	// parse / set socket timeout.
	l.socketTimeout = DefaulSocketTimeout * time.Second

	if socketTimeout, ok := conf["timeout"]; ok {
		num, err := strconv.ParseInt(socketTimeout, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid timeout specification: %s", err.Error())
		}

		l.socketTimeout = time.Duration(num) * time.Second
	}

	// parse / set allowed hosts
	if allowed, ok := conf["allowed hosts"]; ok {
		for _, allow := range strings.Split(allowed, ",") {
			allow = strings.TrimSpace(allow)
			if allow == "" {
				continue
			}

			l.allowedHosts = append(l.allowedHosts, NewAllowedHost(allow))
		}
	}

	// parse / set cache allowed hosts
	l.cacheAllowedHosts = true
	if cache, ok := conf["cache allowed hosts"]; ok {
		l.cacheAllowedHosts = cache == "1"
	}

	return nil
}

// Stop shuts down current listener.
func (l *Listener) Stop() {
	if l.listen != nil {
		l.listen.Close()
	}

	l.listen = nil
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

	listen, err := net.Listen("tcp", l.BindString())
	if err != nil {
		return fmt.Errorf("listen failed: %s", err.Error())
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
				log.Warnf("accept failed: %s", err.Error())

				continue
			}

			log.Infof("stopping %s listener on %s", l.connType, l.BindString())

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

	defaultAdded := false

	// Wrap handler to apply netfilter and logger.
	mappings := handler.GetMappings(l.snc)
	for _, mapping := range mappings {
		mux.Handle(mapping.URL, l.WrapHTTPHandler(mapping.Handler))

		if mapping.URL == "/" {
			defaultAdded = true
		}
	}

	if !defaultAdded {
		mux.Handle("/", l.WrapHTTPHandler(new(ErrorHTTPHandler)))
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
	startTime := time.Now()

	log.Debugf("incoming %s connection from %s", l.connType, remoteAddr)

	if !l.CheckAllowedHosts(remoteAddr) {
		log.Warnf("%s connection from %s prohibited by allowed hosts", l.connType, remoteAddr)

		return
	}

	serveCB()

	log.Debugf("%s connection from %s finished in %9s", l.connType, remoteAddr, time.Since(startTime))
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

type WrappedHTTPHandler struct {
	listener *Listener
	handle   http.Handler
}

func (w *WrappedHTTPHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	reqStr, err := httputil.DumpRequest(req, true)
	if err != nil {
		log.Tracef("%s", err.Error())
	} else {
		log.Tracef("%s", string(reqStr))
	}

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

type ErrorHTTPHandler struct{}

func (w *ErrorHTTPHandler) ServeHTTP(_ http.ResponseWriter, req *http.Request) {
	log.Warnf("unknown url %s requested from %s", req.RequestURI, req.RemoteAddr)
}

type AllowedHost struct {
	Prefix       *netip.Prefix
	IP           *netip.Addr
	HostName     *string
	ResolveCache []netip.Addr
}

func NewAllowedHost(name string) AllowedHost {
	allowed := AllowedHost{}

	if strings.HasPrefix(name, "[") && strings.HasSuffix(name, "]") {
		name = strings.TrimPrefix(name, "[")
		name = strings.TrimSuffix(name, "]")
	}

	// is it a netrange?
	netRange, err := netip.ParsePrefix(name)
	if err == nil {
		allowed.Prefix = &netRange

		return allowed
	}

	// is it an ip address ipv4/ipv6
	if ip, err := netip.ParseAddr(name); err == nil {
		allowed.IP = &ip

		return allowed
	}

	allowed.HostName = &name

	return allowed
}

func (a *AllowedHost) String() string {
	switch {
	case a.Prefix != nil:
		return a.Prefix.String()
	case a.IP != nil:
		return a.IP.String()
	case a.HostName != nil:
		return *a.HostName
	}

	return ""
}

func (a *AllowedHost) Contains(addr netip.Addr, useCaching bool) bool {
	switch {
	case a.Prefix != nil:
		return a.Prefix.Contains(addr)
	case a.IP != nil:
		return a.IP.Compare(addr) == 0
	case a.HostName != nil:
		resolved := a.ResolveCache

		if useCaching || len(a.ResolveCache) == 0 {
			resolved = a.resolveCache()
			if useCaching {
				a.ResolveCache = resolved
			}
		}

		for _, i := range resolved {
			if i.Compare(addr) == 0 {
				return true
			}
		}

		return false
	}

	return false
}

func (a *AllowedHost) resolveCache() []netip.Addr {
	resolved := make([]netip.Addr, 0)

	ips, err := net.LookupIP(*a.HostName)
	if err != nil {
		log.Debugf("dns lookup for %s failed: %s", *a.HostName, err.Error())

		return resolved
	}

	for _, v := range ips {
		i, err := netip.ParseAddr(v.String())
		if err != nil {
			resolved = append(resolved, i)
		}
	}

	return resolved
}
