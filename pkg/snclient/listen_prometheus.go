package snclient

import (
	"encoding/json"
	"net/http"
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func init() {
	RegisterModule(&AvailableListeners, "PrometheusServer", "/settings/Prometheus/server", NewHandlerPrometheus)
}

var (
	prometheusRegistered bool

	promInfoCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "snclient_info",
			Help: "information about this agent",
		},
		[]string{"version", "build", "os"})

	promHTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "total http requests",
		},
		[]string{"code", "path"})

	promHTTPDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "http_response_time_seconds",
		Help: "Duration of HTTP requests.",
	}, []string{"code", "path"})

	promTCPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tcp_requests_total",
			Help: "total tcp requests",
		},
		[]string{"module"})

	promTCPDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "tcp_response_time_seconds",
		Help: "Duration of TCP requests.",
	}, []string{"module"})

	promCollectors = []prometheus.Collector{
		promInfoCount,
		promHTTPRequestsTotal,
		promHTTPDuration,
		promTCPRequestsTotal,
		promTCPDuration,
	}
)

type HandlerPrometheus struct {
	noCopy   noCopy
	handler  http.Handler
	listener *Listener
	password string
	snc      *Agent
}

func NewHandlerPrometheus() Module {
	promHandler := promhttp.InstrumentMetricHandler(
		prometheus.DefaultRegisterer,
		promhttp.HandlerFor(
			prometheus.DefaultGatherer,
			promhttp.HandlerOpts{EnableOpenMetrics: true},
		),
	)

	listen := &HandlerPrometheus{}
	listen.handler = http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if !verifyRequestPassword(listen.snc, req, listen.password) {
			http.Error(res, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			res.Header().Set("Content-Type", "application/json")
			LogError(json.NewEncoder(res).Encode(map[string]interface{}{
				"error": "permission denied",
			}))

			return
		}

		promHandler.ServeHTTP(res, req)
	})

	return listen
}

func (l *HandlerPrometheus) Type() string {
	return "prometheus"
}

func (l *HandlerPrometheus) BindString() string {
	return l.listener.BindString()
}

func (l *HandlerPrometheus) Listener() *Listener {
	return l.listener
}

func (l *HandlerPrometheus) Start() error {
	return l.listener.Start()
}

func (l *HandlerPrometheus) Stop() {
	l.listener.Stop()
}

func (l *HandlerPrometheus) Defaults() ConfigData {
	defaults := ConfigData{
		"port":    "9999",
		"use ssl": "0",
	}
	defaults.Merge(DefaultListenHTTPConfig)

	return defaults
}

func (l *HandlerPrometheus) Init(snc *Agent, conf *ConfigSection, _ *Config, set *ModuleSet) error {
	l.snc = snc
	l.password = DefaultPassword
	if password, ok := conf.GetString("password"); ok {
		l.password = password
	}
	registerMetrics()
	if Revision != "" {
		promInfoCount.WithLabelValues(VERSION+"."+Revision, Build, runtime.GOOS).Set(1)
	} else {
		promInfoCount.WithLabelValues(VERSION, Build, runtime.GOOS).Set(1)
	}

	listener, err := SharedWebListener(snc, conf, l, set)
	if err != nil {
		return err
	}
	l.listener = listener

	return nil
}

func (l *HandlerPrometheus) GetMappings(*Agent) []URLMapping {
	return []URLMapping{
		{URL: "/metrics", Handler: l.handler},
	}
}

func registerMetrics() {
	// registering twice will throw lots of errors
	if prometheusRegistered {
		return
	}

	prometheusRegistered = true

	// register the metrics
	for _, c := range promCollectors {
		if err := prometheus.Register(c); err != nil {
			log.Errorf("failed to register prometheus metric: %s", err.Error())
		}
	}
}
