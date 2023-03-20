package snclient

import (
	"fmt"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	infoCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "snclient_info",
			Help: "information about this agent",
		},
		[]string{"version", "identifier"})
)

func startPrometheus(config *configurationStruct) (prometheusListener *net.Listener) {
	registerMetrics()
	infoCount.WithLabelValues(VERSION, config.identifier).Set(1)

	if config.prometheusServer == "" {
		return
	}

	l, err := net.Listen("tcp", config.prometheusServer)
	if err != nil {
		logger.Fatalf("starting prometheus exporter failed: %s", err)
	}
	prometheusListener = &l
	go func() {
		// make sure we log panics properly
		defer logPanicExit()
		mux := http.NewServeMux()
		handler := promhttp.InstrumentMetricHandler(
			prometheus.DefaultRegisterer, promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{EnableOpenMetrics: true}),
		)
		mux.Handle("/metrics", handler)
		http.Serve(l, mux)
		logger.Debugf("prometheus listener %s stopped", config.prometheusServer)
	}()
	logger.Debugf("serving prometheus metrics at %s/metrics", config.prometheusServer)
	return
}

var prometheusRegistered bool

func registerMetrics() {
	// registering twice will throw lots of errors
	if prometheusRegistered {
		return
	}
	prometheusRegistered = true

	// register the metrics
	if err := prometheus.Register(infoCount); err != nil {
		fmt.Println(err)
	}
}
