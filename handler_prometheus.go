package snclient

import (
	"bufio"
	"fmt"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var (
	prometheusRegistered bool

	infoCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "snclient_info",
			Help: "information about this agent",
		},
		[]string{"version", "build"})
)

type HandlerPrometheus struct {
	noCopy  noCopy
	handler http.Handler
}

func NewHandlerPrometheus() *HandlerPrometheus {
	l := &HandlerPrometheus{
		handler: promhttp.InstrumentMetricHandler(
			prometheus.DefaultRegisterer,
			promhttp.HandlerFor(
				prometheus.DefaultGatherer,
				promhttp.HandlerOpts{EnableOpenMetrics: true},
			),
		),
	}

	return l
}

func (l *HandlerPrometheus) Type() string {
	return "prometheus"
}

func (l *HandlerPrometheus) Defaults() map[string]string {
	defaults := map[string]string{
		"port":           "9999",
		"socket_timeout": "30",
	}

	return defaults
}

func (l *HandlerPrometheus) Init(snc *SNClientInstance) error {
	registerMetrics()
	infoCount.WithLabelValues(VERSION, snc.Build).Set(1)

	/*
		go func() {
			// make sure we log panics properly
			//defer logPanicExit()
			mux := http.NewServeMux()
			mux.Handle("/metrics", handler)
			http.Serve(l, mux)
			//log.Debugf("prometheus listener %s stopped", config.prometheusServer)
		}()
		//log.Debugf("serving prometheus metrics at %s/metrics", config.prometheusServer)
	*/

	return nil
}

func (l *HandlerPrometheus) Handle(snc *SNClientInstance, con net.Conn) {
	defer con.Close()

	req, err := http.ReadRequest(bufio.NewReader(con))
	if err != nil {
		log.Errorf("failed to parse request: %w: %s", err, err.Error())
	}

	log.Tracef("%s %s", req.Method, req.RequestURI)

	if req.RequestURI != "/metrics" {
		fmt.Fprintf(con, "HTTP/1.1 400 Bad Request")

		return
	}

	writer := &resWriter{
		header: make(http.Header, 0),
		con:    con,
	}

	l.handler.ServeHTTP(writer, req)
}

type resWriter struct {
	header http.Header
	con    net.Conn
}

func (r *resWriter) Header() http.Header {
	return r.header
}

func (r *resWriter) Write(b []byte) (int, error) {
	return r.con.Write(b)
}

func (r *resWriter) WriteHeader(status int) {
	fmt.Fprintf(r.con, "HTTP/1.1 %d\n", status)
	r.header.Write(r.con)
	fmt.Fprintf(r.con, "\n\n")
}

func registerMetrics() {
	// registering twice will throw lots of errors
	if prometheusRegistered {
		return
	}

	prometheusRegistered = true

	// register the metrics
	if err := prometheus.Register(infoCount); err != nil {
		log.Errorf("failed to register prometheus metric: %w: %s", err, err.Error())
	}
}
