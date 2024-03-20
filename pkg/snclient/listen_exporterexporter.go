package snclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func init() {
	RegisterModule(&AvailableListeners, "ExporterExporterServer", "/settings/ExporterExporter/server", NewHandlerExporterExporter)
}

type HandlerExporterExporter struct {
	noCopy        noCopy
	handler       http.Handler
	listener      *Listener
	urlPrefix     string
	password      string
	defaultModule string
	snc           *Agent
	modules       map[string]*exporterModuleConfig
	allowedHosts  *AllowedHostConfig
}

// ensure we fully implement the RequestHandlerHTTP type
var _ RequestHandlerHTTP = &HandlerExporterExporter{}

func NewHandlerExporterExporter() Module {
	l := &HandlerExporterExporter{}
	l.handler = &HandlerWebExporterExporter{Handler: l}

	return l
}

func (l *HandlerExporterExporter) Type() string {
	return "exporterexporter"
}

func (l *HandlerExporterExporter) BindString() string {
	return l.listener.BindString()
}

func (l *HandlerExporterExporter) Listener() *Listener {
	return l.listener
}

func (l *HandlerExporterExporter) Start() error {
	return l.listener.Start()
}

func (l *HandlerExporterExporter) Stop() {
	l.listener.Stop()
}

func (l *HandlerExporterExporter) Defaults(runSet *AgentRunSet) ConfigData {
	defaults := ConfigData{
		"port":       "8443",
		"use ssl":    "1",
		"url prefix": "/",
	}
	defaults.Merge(runSet.config.Section("/settings/default").data)
	defaults.Merge(DefaultListenHTTPConfig)

	return defaults
}

func (l *HandlerExporterExporter) Init(snc *Agent, conf *ConfigSection, _ *Config, runSet *AgentRunSet) error {
	l.snc = snc
	l.password = DefaultPassword
	if password, ok := conf.GetString("password"); ok {
		l.password = password
	}

	listener, err := SharedWebListener(snc, conf, l, runSet)
	if err != nil {
		return err
	}
	l.listener = listener
	urlPrefix, _ := conf.GetString("url prefix")
	l.urlPrefix = strings.TrimSuffix(urlPrefix, "/")

	defaultModule, _ := conf.GetString("default module")
	l.defaultModule = defaultModule

	l.modules = map[string]*exporterModuleConfig{}
	moduleDir, _ := conf.GetString("modules dir")
	if moduleDir != "" {
		modules, err2 := l.readModules(snc, moduleDir)
		if err2 != nil {
			return err2
		}
		l.modules = modules
	}

	allowedHosts, err2 := NewAllowedHostConfig(conf)
	if err2 != nil {
		return err2
	}
	l.allowedHosts = allowedHosts

	return nil
}

func (l *HandlerExporterExporter) GetAllowedHosts() *AllowedHostConfig {
	return l.allowedHosts
}

func (l *HandlerExporterExporter) CheckPassword(req *http.Request, _ URLMapping) bool {
	return verifyRequestPassword(l.snc, req, l.password)
}

func (l *HandlerExporterExporter) GetMappings(*Agent) []URLMapping {
	return []URLMapping{
		{URL: l.urlPrefix + "/list", Handler: l.handler},
		{URL: l.urlPrefix + "/proxy", Handler: l.handler},
	}
}

func (l *HandlerExporterExporter) readModules(snc *Agent, moduleDir string) (map[string]*exporterModuleConfig, error) {
	modules := map[string]*exporterModuleConfig{}

	mfs, err := os.ReadDir(moduleDir)
	if err != nil {
		return nil, fmt.Errorf("failed reading directory: %s, %s", moduleDir, err.Error())
	}

	yamlSuffixes := map[string]bool{
		".yml":  true,
		".yaml": true,
	}
	for _, entry := range mfs {
		fullpath := filepath.Join(moduleDir, entry.Name())
		if entry.IsDir() || !yamlSuffixes[filepath.Ext(entry.Name())] {
			log.Warnf("skipping non-yaml file %v", fullpath)

			continue
		}

		if err := modulesAdd(snc, modules, entry, fullpath); err != nil {
			return nil, err
		}
	}

	return modules, nil
}

func modulesAdd(snc *Agent, modules map[string]*exporterModuleConfig, entry fs.DirEntry, fullpath string) error {
	moduleName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
	if _, ok := modules[moduleName]; ok {
		return fmt.Errorf("module %s is already defined", moduleName)
	}
	file, err := os.Open(fullpath)
	if err != nil {
		return fmt.Errorf("failed to open config file %s, %w", fullpath, err)
	}
	defer file.Close()

	mcfg, err := readModuleConfig(moduleName, file)
	if err != nil {
		return fmt.Errorf("failed reading configs %s, %w", fullpath, err)
	}
	mcfg.snc = snc

	if mcfg.Timeout == 0 {
		mcfg.Timeout = DefaultSocketTimeout * time.Second
	}

	log.Debugf("read exporter module config '%s' from: %s", moduleName, fullpath)
	modules[moduleName] = mcfg

	return nil
}

type HandlerWebExporterExporter struct {
	noCopy  noCopy
	Handler *HandlerExporterExporter
}

func (l *HandlerWebExporterExporter) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/list":
		l.Handler.listModules(res, req)
	case "/proxy":
		l.Handler.doProxy(res, req)
	default:
		res.WriteHeader(http.StatusNotFound)
		LogError2(res.Write([]byte("404 - nothing here\n")))
	}
}

func (l *HandlerExporterExporter) listModules(res http.ResponseWriter, req *http.Request) {
	switch req.Header.Get("Accept") {
	case "application/json":
		log.Debugf("Listing modules in json")
		moduleJSON, err := json.Marshal(l.modules) //nolint:musttag // no idea what this linter wants to have tagged
		if err != nil {
			log.Error(err)
			http.Error(res, "Failed to produce JSON", http.StatusInternalServerError)

			return
		}
		res.WriteHeader(http.StatusOK)
		res.Header().Set("Content-Type", "application/json")
		LogError2(res.Write(moduleJSON))
	default:
		log.Debugf("Listing modules in html")
		res.WriteHeader(http.StatusOK)
		res.Header().Set("Content-Type", "text/html; charset=utf-8")
		LogError2(res.Write([]byte("<h2>Exporters:</h2>\n")))
		LogError2(res.Write([]byte("<ul>\n")))
		for name := range l.modules {
			LogError2(fmt.Fprintf(res, "<li><a href=\"/proxy?module=%s\">%s</a></li>\n", name, name))
		}
		LogError2(res.Write([]byte("</ul>\n")))
	}
}

func (l *HandlerExporterExporter) doProxy(res http.ResponseWriter, req *http.Request) {
	mod, ok := req.URL.Query()["module"]
	if !ok && l.defaultModule == "" {
		log.Errorf("no module given")
		http.Error(res, fmt.Sprintf("require parameter module is missing%v\n", mod), http.StatusBadRequest)

		return
	}

	if len(mod) == 0 {
		mod = append(mod, l.defaultModule)
	}

	log.Debugf("running module %v\n", mod[0])

	mcfg, ok := l.modules[mod[0]]
	if !ok {
		log.Warnf("unknown module requested  %v\n", mod)
		http.Error(res, fmt.Sprintf("unknown module %v\n", mod), http.StatusNotFound)

		return
	}

	mcfg.ServeHTTP(res, req)
}

type exporterModuleConfig struct {
	Method  string                 `json:"method"  yaml:"method"`
	Timeout time.Duration          `json:"timeout" yaml:"timeout"`
	XXX     map[string]interface{} `json:",inline" yaml:",inline"`

	Exec exporterExecConfig `json:"exec" yaml:"exec"`
	HTTP exporterHTTPConfig `json:"http" yaml:"http"`
	File exporterFileConfig `json:"file" yaml:"file"`

	snc  *Agent
	name string
}

type exporterHTTPConfig struct {
	Verify                 bool                   `yaml:"verify"`                   // false, not implemented
	TLSInsecureSkipVerify  bool                   `yaml:"tls_insecure_skip_verify"` // false
	TLSCertFile            *string                `yaml:"tls_cert_file"`            // no default
	TLSKeyFile             *string                `yaml:"tls_key_file"`             // no default
	TLSCACertFile          *string                `yaml:"tls_ca_cert_file"`         // no default
	Port                   int                    `yaml:"port"`                     // no default
	Path                   string                 `yaml:"path"`                     // /metrics
	Scheme                 string                 `yaml:"scheme"`                   // http
	Address                string                 `yaml:"address"`                  // localhost
	Headers                map[string]string      `yaml:"headers"`                  // no default
	BasicAuthUsername      string                 `yaml:"basic_auth_username"`      // no default
	BasicAuthPassword      string                 `yaml:"basic_auth_password"`      // no default
	XXX                    map[string]interface{} `yaml:",inline"`
	tlsConfig              *tls.Config
	mcfg                   *exporterModuleConfig
	*httputil.ReverseProxy `json:"-"`
}

type exporterExecConfig struct {
	Command string                 `yaml:"command"`
	Args    []string               `yaml:"args"`
	Env     map[string]string      `yaml:"env"`
	XXX     map[string]interface{} `yaml:",inline"`
	mcfg    *exporterModuleConfig
}

type exporterFileConfig struct {
	Path string                 `yaml:"path"`
	XXX  map[string]interface{} `yaml:",inline"`
	mcfg *exporterModuleConfig
}

func readModuleConfig(name string, r io.Reader) (*exporterModuleConfig, error) {
	buf := bytes.Buffer{}
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, fmt.Errorf("io.Copy: %s", err.Error())
	}
	cfg := exporterModuleConfig{}

	if err := yaml.Unmarshal(buf.Bytes(), &cfg); err != nil {
		return nil, fmt.Errorf("yaml.Unmarshal: %s", err.Error())
	}

	if err := checkModuleConfig(name, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func checkModuleConfig(name string, cfg *exporterModuleConfig) error {
	if len(cfg.XXX) != 0 {
		return fmt.Errorf("unknown module configuration fields: %v", cfg.XXX)
	}

	cfg.name = name

	switch cfg.Method {
	case "http":
		if len(cfg.HTTP.XXX) != 0 {
			return fmt.Errorf("unknown http module configuration fields: %v", cfg.HTTP.XXX)
		}

		if cfg.HTTP.Port == 0 {
			return fmt.Errorf("module %v must have a non-zero port set", name)
		}
		if cfg.HTTP.Verify {
			log.Warnf("module %v uses verify=true which is not implemented", name)
		}
		if cfg.HTTP.Scheme == "" {
			cfg.HTTP.Scheme = "http"
		}
		if cfg.HTTP.Path == "" {
			cfg.HTTP.Path = "/metrics"
		}
		if cfg.HTTP.Address == "" {
			cfg.HTTP.Address = "localhost"
		}

		tlsConfig, err := cfg.HTTP.getTLSConfig()
		if err != nil {
			return fmt.Errorf("could not create tls config, %w", err)
		}

		dirFunc, err := cfg.getReverseProxyDirectorFunc()
		if err != nil {
			return err
		}

		cfg.HTTP.tlsConfig = tlsConfig
		cfg.HTTP.ReverseProxy = &httputil.ReverseProxy{
			Transport:    &http.Transport{TLSClientConfig: tlsConfig},
			Director:     dirFunc,
			ErrorHandler: getReverseProxyErrorHandlerFunc(cfg.name),
		}
	case "exec":
		if len(cfg.Exec.XXX) != 0 {
			return fmt.Errorf("unknown exec module configuration fields: %v", cfg.Exec.XXX)
		}
	case "file":
		if cfg.File.Path == "" {
			return fmt.Errorf("path argument for file module is mandatory")
		}
	default:
		return fmt.Errorf("unknown module method: %v", cfg.Method)
	}

	return nil
}

func (cfg *exporterHTTPConfig) getTLSConfig() (*tls.Config, error) {
	config := &tls.Config{
		InsecureSkipVerify: cfg.TLSInsecureSkipVerify, //nolint:gosec // set by config but default is false
		MinVersion:         tls.VersionTLS12,
	}
	if cfg.TLSCACertFile != nil {
		caCert, err := os.ReadFile(*cfg.TLSCACertFile)
		if err != nil {
			return nil, fmt.Errorf("could not read ca from %v, %w", *cfg.TLSCACertFile, err)
		}

		config.ClientCAs = x509.NewCertPool()
		config.ClientCAs.AppendCertsFromPEM(caCert)
	}
	if cfg.TLSCertFile != nil && cfg.TLSKeyFile != nil {
		cert, err := tls.LoadX509KeyPair(*cfg.TLSCertFile, *cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed reading TLS credentials, %w", err)
		}
		config.Certificates = []tls.Certificate{cert}
	}

	return config, nil
}

func (cfg *exporterModuleConfig) getReverseProxyDirectorFunc() (func(*http.Request), error) {
	base, err := url.Parse(cfg.HTTP.Path)
	if err != nil {
		return nil, fmt.Errorf("http configuration path should be a valid URL path with options, %w", err)
	}

	cvs := base.Query()

	return func(req *http.Request) {
		qvs := req.URL.Query()
		for k, vs := range cvs {
			for _, v := range vs {
				qvs.Add(k, v)
			}
		}
		qvs["module"] = qvs["module"][1:]

		req.URL.RawQuery = qvs.Encode()

		for k, v := range cfg.HTTP.Headers {
			req.Header.Add(k, v)
		}

		req.URL.Scheme = cfg.HTTP.Scheme
		req.URL.Host = net.JoinHostPort(cfg.HTTP.Address, strconv.Itoa(cfg.HTTP.Port))
		if _, ok := cfg.HTTP.Headers["host"]; ok {
			req.Host = cfg.HTTP.Headers["host"]
		}
		req.URL.Path = base.Path
		if cfg.HTTP.BasicAuthUsername != "" && cfg.HTTP.BasicAuthPassword != "" {
			req.SetBasicAuth(cfg.HTTP.BasicAuthUsername, cfg.HTTP.BasicAuthPassword)
		}
	}, nil
}

func getReverseProxyErrorHandlerFunc(name string) func(http.ResponseWriter, *http.Request, error) {
	return func(res http.ResponseWriter, _ *http.Request, err error) {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Errorf("Request time out for module '%s'", name)
			http.Error(res, http.StatusText(http.StatusGatewayTimeout), http.StatusGatewayTimeout)

			return
		}

		log.Errorf("Proxy error for module '%s': %v", name, err)
		http.Error(res, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
	}
}

func (cfg *exporterModuleConfig) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	wrapReq := req
	cancel := func() {}
	if cfg.Timeout != 0 {
		log.Debugf("setting module %v timeout to %v", cfg.name, cfg.Timeout)

		var ctx context.Context
		ctx, cancel = context.WithTimeout(req.Context(), cfg.Timeout)
		wrapReq = req.WithContext(ctx)
	}
	defer cancel()

	switch cfg.Method {
	case "exec":
		cfg.Exec.mcfg = cfg
		cfg.Exec.ServeHTTP(res, wrapReq)
	case "http":
		cfg.HTTP.mcfg = cfg
		cfg.HTTP.ServeHTTP(res, wrapReq)
	case "file":
		cfg.File.mcfg = cfg
		cfg.File.ServeHTTP(res, wrapReq)
	default:
		log.Errorf("unknown module method  %v\n", cfg.Method)
		http.Error(res, fmt.Sprintf("unknown module method %v\n", cfg.Method), http.StatusNotFound)

		return
	}
}

func (m exporterFileConfig) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	deadline, _ := ctx.Deadline()
	data, mtime, err := readFileWithDeadline(m.Path, deadline)
	if err != nil {
		http.Error(res, fmt.Sprintf("file module error: %s\n", err.Error()), http.StatusInternalServerError)

		return
	}

	data = bytes.TrimSpace(data)
	res.WriteHeader(http.StatusOK)
	LogError2(res.Write(data))
	LogError2(res.Write([]byte("\n")))

	// add mtime metric
	if !mtime.IsZero() {
		LogError2(res.Write([]byte("# HELP expexp_file_mtime_timestamp Time of modification of parsed file\n")))
		LogError2(res.Write([]byte("# TYPE expexp_file_mtime_timestamp gauge\n")))
		LogError2(fmt.Fprintf(res, "expexp_file_mtime_timestamp{module=\"%s\",path=\"%s\"} %d\n", m.mcfg.name, m.Path, mtime.Unix()))
	}
}

func readFileWithDeadline(path string, deadline time.Time) ([]byte, time.Time, error) {
	file, err := os.Open(path)
	mtime := time.Time{}
	if err != nil {
		return nil, mtime, fmt.Errorf("open %s: %s", path, err.Error())
	}
	defer file.Close()

	if !deadline.IsZero() {
		if err2 := file.SetDeadline(deadline); err2 != nil {
			return nil, mtime, fmt.Errorf("file.SetDeadline %s: %s", path, err2.Error())
		}
	}

	if info, err2 := file.Stat(); err2 == nil {
		if info.Mode().IsRegular() {
			mtime = info.ModTime()
		}
	}
	data, err := io.ReadAll(file)

	return data, mtime, fmt.Errorf("io.read %s: %s", path, err.Error())
}

func (m exporterExecConfig) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	cmd, err := m.mcfg.snc.MakeCmd(ctx, m.Command)
	if err != nil {
		http.Error(res, fmt.Sprintf("exec module error: %s\n", err.Error()), http.StatusInternalServerError)

		return
	}
	cmd.Path = m.Command
	cmd.Args = m.Args
	for k, v := range m.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(DefaultSocketTimeout * time.Second)
	}
	stdout, stderr, _, _, err := m.mcfg.snc.runExternalCommand(ctx, cmd, deadline.Unix())
	if err != nil {
		http.Error(res, fmt.Sprintf("exec module error: %s\n", err.Error()), http.StatusInternalServerError)

		return
	}
	if stderr != "" {
		log.Warnf("expexp module %s stderr: %s", m.mcfg.name, stderr)
	}

	res.WriteHeader(http.StatusOK)
	LogError2(res.Write([]byte(stdout)))
	LogError2(res.Write([]byte("\n")))
}
